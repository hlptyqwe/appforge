package logic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/agentpki"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	localAgentPending         = int64(core.LocalAgentStatus_LOCAL_AGENT_STATUS_PENDING)
	localAgentOnline          = int64(core.LocalAgentStatus_LOCAL_AGENT_STATUS_ONLINE)
	localAgentOffline         = int64(core.LocalAgentStatus_LOCAL_AGENT_STATUS_OFFLINE)
	localAgentRevoked         = int64(core.LocalAgentStatus_LOCAL_AGENT_STATUS_REVOKED)
	localAgentUpgradeRequired = int64(core.LocalAgentStatus_LOCAL_AGENT_STATUS_UPGRADE_REQUIRED)
	localAgentAccepting       = int64(core.LocalAgentDrainStatus_LOCAL_AGENT_DRAIN_STATUS_ACCEPTING)
	localCertificateActive    = int64(core.LocalAgentCertificateStatus_LOCAL_AGENT_CERTIFICATE_STATUS_ACTIVE)
	localCertificateRotated   = int64(core.LocalAgentCertificateStatus_LOCAL_AGENT_CERTIFICATE_STATUS_ROTATED)
	localCertificateRevoked   = int64(core.LocalAgentCertificateStatus_LOCAL_AGENT_CERTIFICATE_STATUS_REVOKED)
	localCertificateExpired   = int64(core.LocalAgentCertificateStatus_LOCAL_AGENT_CERTIFICATE_STATUS_EXPIRED)
	localRegistrationPending  = int64(1)
	localRegistrationUsed     = int64(2)
	localRegistrationRevoked  = int64(4)
	localProtocolCurrent      = int32(2)
	localProtocolMinimum      = int32(1)
	hybridArtifactVerified    = int64(2)
)

var (
	localAgentCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)
	sha256Pattern         = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

const localAgentSelect = `SELECT id,tenant_id,agent_code,agent_name,pool_code,status,drain_status,
protocol_version,agent_version,artifact_mode,customer_storage_ref,allowed_app_ids,certificate_serial,
last_nonce,last_request_at,last_heartbeat_at,create_by,create_time,update_time FROM t_local_agent`

const localCertificateSelect = `SELECT id,tenant_id,agent_id,serial_number,fingerprint_sha256,
certificate_pem,status,not_before,not_after,revoked_at,revoke_reason,create_time,update_time
FROM t_local_agent_certificate`

func createLocalAgentRegistration(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CreateLocalAgentRegistrationReq) (*core.LocalAgentRegistrationResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	code := strings.TrimSpace(in.AgentCode)
	if !localAgentCodePattern.MatchString(code) {
		return nil, status.Error(codes.InvalidArgument, "agent_code must use 2-64 lowercase letters, digits, underscores or hyphens")
	}
	if err := requireText(in.AgentName, "agent_name", 128); err != nil {
		return nil, err
	}
	pool, err := normalizedBuildPool(in.PoolCode)
	if err != nil {
		return nil, err
	}
	if in.ArtifactMode < core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE || in.ArtifactMode > core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_AIR_GAPPED {
		return nil, status.Error(codes.InvalidArgument, "artifact_mode is invalid")
	}
	if in.ArtifactMode == core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE && strings.TrimSpace(in.CustomerStorageRef) == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_storage_ref is required for customer storage")
	}
	if err := requireOptionalText(in.CustomerStorageRef, "customer_storage_ref", 500); err != nil {
		return nil, err
	}
	appIDs, appJSON, err := normalizeAgentApps(ctx, svcCtx.DB, tenant, in.AllowedAppIds)
	if err != nil {
		return nil, err
	}
	capabilities, err := normalizeAgentCapabilities(in.Capabilities)
	if err != nil {
		return nil, err
	}
	ttl := in.ExpiresSeconds
	if ttl <= 0 {
		ttl = 900
	}
	if ttl > 86400 {
		return nil, status.Error(codes.InvalidArgument, "expires_seconds must not exceed 86400")
	}
	token, tokenHash, err := newRegistrationToken()
	if err != nil {
		return nil, billingInternalError("generate Local Agent registration token", err)
	}
	expires := billingNow().Add(time.Duration(ttl) * time.Second)
	var agent models.TLocalAgent
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var existingID int64
		err := session.QueryRowCtx(txCtx, &existingID, `SELECT id FROM t_local_agent WHERE tenant_id=? AND agent_code=? FOR UPDATE`, tenant, code)
		if err == nil {
			return status.Error(codes.AlreadyExists, "agent_code already exists")
		}
		if err != sqlx.ErrNotFound && err != sql.ErrNoRows {
			return err
		}
		result, err := session.ExecCtx(txCtx, `INSERT INTO t_local_agent
(tenant_id,agent_code,agent_name,pool_code,status,drain_status,protocol_version,agent_version,
artifact_mode,customer_storage_ref,allowed_app_ids,create_by) VALUES (?,?,?,?,?,?,?,?,?,NULLIF(?,''),?,?)`,
			tenant, code, strings.TrimSpace(in.AgentName), pool, localAgentPending, localAgentAccepting,
			localProtocolCurrent, "", int64(in.ArtifactMode), strings.TrimSpace(in.CustomerStorageRef), appJSON, actorID(ctx))
		if err != nil {
			return err
		}
		agentID, _ := result.LastInsertId()
		if _, err := session.ExecCtx(txCtx, `INSERT INTO t_local_agent_registration
(tenant_id,agent_id,token_hash,status,expires_at,create_by) VALUES (?,?,?,?,?,?)`, tenant, agentID,
			tokenHash, localRegistrationPending, expires, actorID(ctx)); err != nil {
			return err
		}
		if err := replaceAgentCapabilities(txCtx, session, tenant, agentID, capabilities); err != nil {
			return err
		}
		return session.QueryRowCtx(txCtx, &agent, localAgentSelect+` WHERE id=?`, agentID)
	})
	if err != nil {
		return nil, err
	}
	mapped, err := mapLocalAgent(ctx, svcCtx.DB, &agent)
	if err != nil {
		return nil, err
	}
	mapped.AllowedAppIds = appIDs
	return &core.LocalAgentRegistrationResp{Base: okBase(), Data: mapped, RegistrationToken: token, ExpiresAt: millis(expires)}, nil
}

func registerLocalAgent(ctx context.Context, svcCtx *svc.ServiceContext, in *core.RegisterLocalAgentReq) (*core.RegisterLocalAgentResp, error) {
	if in == nil || strings.TrimSpace(in.RegistrationToken) == "" {
		return nil, status.Error(codes.InvalidArgument, "registration_token is required")
	}
	if err := validateAgentClockNonce(in.Timestamp, in.Nonce); err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(strings.TrimSpace(in.RegistrationToken)))
	tokenHash := hex.EncodeToString(digest[:])
	var agent models.TLocalAgent
	var registration struct {
		ID        int64     `db:"id"`
		TenantID  int64     `db:"tenant_id"`
		AgentID   int64     `db:"agent_id"`
		Status    int64     `db:"status"`
		ExpiresAt time.Time `db:"expires_at"`
	}
	if err := svcCtx.DB.QueryRowCtx(ctx, &registration,
		`SELECT id,tenant_id,agent_id,status,expires_at FROM t_local_agent_registration WHERE token_hash=?`, tokenHash); err != nil {
		return nil, status.Error(codes.Unauthenticated, "registration token is invalid")
	}
	if registration.Status != localRegistrationPending || !billingNow().Before(registration.ExpiresAt) {
		return nil, status.Error(codes.Unauthenticated, "registration token is used, expired or revoked")
	}
	certificate, err := svcCtx.AgentPKI.SignCSR(in.CsrPem, registration.TenantID, registration.AgentID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "csr_pem is invalid: %v", err)
	}
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var locked struct {
			Status    int64     `db:"status"`
			ExpiresAt time.Time `db:"expires_at"`
		}
		if err := session.QueryRowCtx(txCtx, &locked,
			`SELECT status,expires_at FROM t_local_agent_registration WHERE id=? FOR UPDATE`, registration.ID); err != nil {
			return err
		}
		if locked.Status != localRegistrationPending || !billingNow().Before(locked.ExpiresAt) {
			return status.Error(codes.Unauthenticated, "registration token is used, expired or revoked")
		}
		if err := session.QueryRowCtx(txCtx, &agent, localAgentSelect+` WHERE id=? AND tenant_id=? FOR UPDATE`, registration.AgentID, registration.TenantID); err != nil {
			return err
		}
		if agent.Status == localAgentRevoked {
			return status.Error(codes.PermissionDenied, "Local Agent is revoked")
		}
		if _, err := session.ExecCtx(txCtx, `INSERT INTO t_local_agent_certificate
(tenant_id,agent_id,serial_number,fingerprint_sha256,certificate_pem,status,not_before,not_after)
VALUES (?,?,?,?,?,?,?,?)`, registration.TenantID, registration.AgentID, certificate.Serial, certificate.Fingerprint, certificate.PEM,
			localCertificateActive, certificate.NotBefore, certificate.NotAfter); err != nil {
			return err
		}
		newStatus := localAgentOnline
		if in.ProtocolVersion < localProtocolMinimum || in.ProtocolVersion > localProtocolCurrent {
			newStatus = localAgentUpgradeRequired
		}
		requestAt := timeFromMillis(in.Timestamp)
		if _, err := session.ExecCtx(txCtx, `UPDATE t_local_agent SET status=?,protocol_version=?,agent_version=?,
certificate_serial=?,last_nonce=?,last_request_at=?,last_heartbeat_at=CURRENT_TIMESTAMP(3) WHERE id=?`,
			newStatus, in.ProtocolVersion, strings.TrimSpace(in.AgentVersion), certificate.Serial,
			strings.TrimSpace(in.Nonce), requestAt, registration.AgentID); err != nil {
			return err
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_local_agent_registration SET status=?,used_at=CURRENT_TIMESTAMP(3),
used_ip=NULLIF(?, '') WHERE id=?`, localRegistrationUsed, strings.TrimSpace(in.SourceIp), registration.ID); err != nil {
			return err
		}
		return session.QueryRowCtx(txCtx, &agent, localAgentSelect+` WHERE id=?`, registration.AgentID)
	})
	if err != nil {
		return nil, err
	}
	return localAgentCertificateResponse(ctx, svcCtx, &agent, certificate)
}

func getLocalAgent(ctx context.Context, svcCtx *svc.ServiceContext, in *core.LocalAgentIdReq) (*core.LocalAgentResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	var item models.TLocalAgent
	if err := svcCtx.DB.QueryRowCtx(ctx, &item, localAgentSelect+` WHERE id=? AND tenant_id=?`, in.Id, tenant); err != nil {
		return nil, notFoundOrInternal(err, "Local Agent")
	}
	mapped, err := mapLocalAgent(ctx, svcCtx.DB, &item)
	return &core.LocalAgentResp{Base: okBase(), Data: mapped}, err
}

func listLocalAgents(ctx context.Context, svcCtx *svc.ServiceContext, in *core.LocalAgentListReq) (*core.LocalAgentListResp, error) {
	if in == nil {
		in = &core.LocalAgentListReq{}
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	page, size := int64(in.Page), int64(in.PageSize)
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	where := "tenant_id=?"
	args := []any{tenant}
	if in.Status != core.LocalAgentStatus_LOCAL_AGENT_STATUS_UNKNOWN {
		where += " AND status=?"
		args = append(args, int64(in.Status))
	}
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, `SELECT COUNT(*) FROM t_local_agent WHERE `+where, args...); err != nil {
		return nil, billingInternalError("count Local Agents", err)
	}
	args = append(args, size, (page-1)*size)
	var items []models.TLocalAgent
	if err := svcCtx.DB.QueryRowsCtx(ctx, &items, localAgentSelect+` WHERE `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...); err != nil {
		return nil, billingInternalError("list Local Agents", err)
	}
	result := make([]*core.LocalAgent, 0, len(items))
	for index := range items {
		mapped, err := mapLocalAgent(ctx, svcCtx.DB, &items[index])
		if err != nil {
			return nil, err
		}
		result = append(result, mapped)
	}
	return &core.LocalAgentListResp{Base: okBase(), Data: result, Total: total}, nil
}

func heartbeatLocalAgent(ctx context.Context, svcCtx *svc.ServiceContext, in *core.HeartbeatLocalAgentReq) (*core.LocalAgentResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	capabilities, err := normalizeAgentCapabilities(in.Capabilities)
	if err != nil {
		return nil, err
	}
	agent, err := authenticateLocalAgent(ctx, svcCtx, in.Auth)
	if err != nil {
		return nil, err
	}
	newStatus := localAgentOnline
	if in.ProtocolVersion < localProtocolMinimum || in.ProtocolVersion > localProtocolCurrent {
		newStatus = localAgentUpgradeRequired
	}
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := replaceAgentCapabilities(txCtx, session, agent.TenantId, agent.Id, capabilities); err != nil {
			return err
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_local_agent SET status=?,protocol_version=?,agent_version=?,
last_heartbeat_at=CURRENT_TIMESTAMP(3) WHERE id=? AND status<>?`, newStatus, in.ProtocolVersion,
			strings.TrimSpace(in.AgentVersion), agent.Id, localAgentRevoked); err != nil {
			return err
		}
		return session.QueryRowCtx(txCtx, agent, localAgentSelect+` WHERE id=?`, agent.Id)
	})
	if err != nil {
		return nil, err
	}
	mapped, err := mapLocalAgent(ctx, svcCtx.DB, agent)
	return &core.LocalAgentResp{Base: okBase(), Data: mapped}, err
}

func drainLocalAgent(ctx context.Context, svcCtx *svc.ServiceContext, in *core.DrainLocalAgentReq) (*core.LocalAgentResp, error) {
	if in == nil || in.Id <= 0 || in.DrainStatus < core.LocalAgentDrainStatus_LOCAL_AGENT_DRAIN_STATUS_ACCEPTING || in.DrainStatus > core.LocalAgentDrainStatus_LOCAL_AGENT_DRAIN_STATUS_DRAINED {
		return nil, status.Error(codes.InvalidArgument, "id or drain_status is invalid")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	if _, err := svcCtx.DB.ExecCtx(ctx, `UPDATE t_local_agent SET drain_status=? WHERE id=? AND tenant_id=? AND status<>?`, int64(in.DrainStatus), in.Id, tenant, localAgentRevoked); err != nil {
		return nil, billingInternalError("drain Local Agent", err)
	}
	return getLocalAgent(ctx, svcCtx, &core.LocalAgentIdReq{Id: in.Id, TenantId: tenant})
}

func revokeLocalAgent(ctx context.Context, svcCtx *svc.ServiceContext, in *core.RevokeLocalAgentReq) (*core.LocalAgentResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	if err := requireText(in.Reason, "reason", 500); err != nil {
		return nil, err
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		result, err := session.ExecCtx(txCtx, `UPDATE t_local_agent SET status=?,drain_status=? WHERE id=? AND tenant_id=?`,
			localAgentRevoked, int64(core.LocalAgentDrainStatus_LOCAL_AGENT_DRAIN_STATUS_DRAINED), in.Id, tenant)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return sqlx.ErrNotFound
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_local_agent_certificate SET status=?,revoked_at=CURRENT_TIMESTAMP(3),
revoke_reason=? WHERE agent_id=? AND tenant_id=? AND status=?`, localCertificateRevoked, strings.TrimSpace(in.Reason), in.Id, tenant, localCertificateActive); err != nil {
			return err
		}
		_, err = session.ExecCtx(txCtx, `UPDATE t_local_agent_registration SET status=? WHERE agent_id=? AND tenant_id=? AND status=?`,
			localRegistrationRevoked, in.Id, tenant, localRegistrationPending)
		return err
	})
	if err != nil {
		return nil, notFoundOrInternal(err, "Local Agent")
	}
	return getLocalAgent(ctx, svcCtx, &core.LocalAgentIdReq{Id: in.Id, TenantId: tenant})
}

func rotateLocalAgentCertificate(ctx context.Context, svcCtx *svc.ServiceContext, in *core.RotateLocalAgentCertificateReq) (*core.RegisterLocalAgentResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	agent, err := authenticateLocalAgent(ctx, svcCtx, in.Auth)
	if err != nil {
		return nil, err
	}
	certificate, err := svcCtx.AgentPKI.SignCSR(in.CsrPem, agent.TenantId, agent.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "csr_pem is invalid: %v", err)
	}
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(txCtx, `UPDATE t_local_agent_certificate SET status=? WHERE agent_id=? AND status=?`,
			localCertificateRotated, agent.Id, localCertificateActive); err != nil {
			return err
		}
		if _, err := session.ExecCtx(txCtx, `INSERT INTO t_local_agent_certificate
(tenant_id,agent_id,serial_number,fingerprint_sha256,certificate_pem,status,not_before,not_after)
VALUES (?,?,?,?,?,?,?,?)`, agent.TenantId, agent.Id, certificate.Serial, certificate.Fingerprint,
			certificate.PEM, localCertificateActive, certificate.NotBefore, certificate.NotAfter); err != nil {
			return err
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_local_agent SET certificate_serial=? WHERE id=?`, certificate.Serial, agent.Id); err != nil {
			return err
		}
		return session.QueryRowCtx(txCtx, agent, localAgentSelect+` WHERE id=?`, agent.Id)
	})
	if err != nil {
		return nil, err
	}
	return localAgentCertificateResponse(ctx, svcCtx, agent, certificate)
}

func claimLocalAgentBuildTask(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ClaimLocalAgentBuildTaskReq) (*core.LocalAgentBuildTaskResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	agent, err := authenticateLocalAgent(ctx, svcCtx, in.Auth)
	if err != nil {
		return nil, err
	}
	if agent.Status != localAgentOnline || agent.DrainStatus != localAgentAccepting {
		return &core.LocalAgentBuildTaskResp{Base: okBase(), ArtifactMode: core.HybridArtifactMode(agent.ArtifactMode)}, nil
	}
	apps := parseAppIDs(agent.AllowedAppIds)
	if len(apps) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "Local Agent has no authorized applications")
	}
	seconds := leaseSeconds(in.LeaseSeconds)
	builderID := fmt.Sprintf("local-%d", agent.Id)
	var claimed *models.TBuildTask
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		subscription, entitlement, _, err := loadTenantBilling(txCtx, session, agent.TenantId, true)
		if err != nil {
			return err
		}
		if !subscriptionAllowsConsumption(subscription, billingNow()) || entitlement.Status != entitlementActive || !billingNow().Before(entitlement.ValidUntil) {
			return nil
		}
		active, err := activeSlotCount(txCtx, session, agent.PoolCode, agent.TenantId, 0)
		if err != nil {
			return err
		}
		if entitlement.MaxBuildConcurrency >= 0 && active >= entitlement.MaxBuildConcurrency {
			return nil
		}
		var nodeActive int64
		if err := session.QueryRowCtx(txCtx, &nodeActive, `SELECT COUNT(*) FROM t_build_slot_lease WHERE node_code=? AND status=? AND lease_until>CURRENT_TIMESTAMP(3)`, builderID, buildSlotActive); err != nil {
			return err
		}
		if nodeActive >= agentMaxConcurrency(ctx, session, agent.Id) {
			return nil
		}
		placeholders := strings.TrimRight(strings.Repeat("?,", len(apps)), ",")
		args := []any{agent.TenantId, agent.PoolCode, buildStatusPending, buildStatusBuilding, buildStatusSigning, buildStatusUploading}
		for _, appID := range apps {
			args = append(args, appID)
		}
		var tasks []models.TBuildTask
		query := buildTaskSelect + ` WHERE tenant_id=? AND pool_code=? AND
(status=? OR (status IN (?,?,?) AND (lease_until IS NULL OR lease_until<CURRENT_TIMESTAMP(3)))) AND app_id IN (` + placeholders + `)
ORDER BY priority DESC,queued_at ASC,id ASC LIMIT 8 FOR UPDATE SKIP LOCKED`
		if err := session.QueryRowsCtx(txCtx, &tasks, query, args...); err != nil {
			return err
		}
		for index := range tasks {
			task := &tasks[index]
			appMax, _, _, err := schedulerPolicy(txCtx, session, task.TenantId, task.AppId, task.PoolCode)
			if err != nil {
				return err
			}
			appActive, err := activeSlotCount(txCtx, session, task.PoolCode, task.TenantId, task.AppId)
			if err != nil {
				return err
			}
			if appActive >= appMax {
				continue
			}
			recovery := task.Status != buildStatusPending
			attempt := task.BuilderAttempt + 1
			result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status=?,builder_id=?,builder_attempt=?,
start_time=COALESCE(start_time,CURRENT_TIMESTAMP(3)),lease_until=DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL ? SECOND)
WHERE id=? AND builder_attempt=?`, buildStatusBuilding, builderID, attempt, seconds, task.Id, task.BuilderAttempt)
			if err != nil {
				return err
			}
			affected, _ := result.RowsAffected()
			if affected != 1 {
				continue
			}
			if _, err := session.ExecCtx(txCtx, `INSERT INTO t_build_slot_lease
(task_id,tenant_id,app_id,node_code,pool_code,builder_attempt,status,lease_until)
VALUES (?,?,?,?,?,?,?,DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL ? SECOND))`, task.Id, task.TenantId, task.AppId, builderID, task.PoolCode, attempt, buildSlotActive, seconds); err != nil {
				return err
			}
			task.Status = buildStatusBuilding
			task.BuilderId = nullString(builderID)
			task.BuilderAttempt = attempt
			if !(task.RetryOfTaskId > 0 && entitlement.ChargeRetryBuild == 0) {
				if err := confirmQuotaInSession(txCtx, session, task.TenantId, "build.count", fmt.Sprintf("build:%d", task.Id), "build.started", task.Id, billingUsageMetadata(map[string]any{"builderAttempt": attempt, "localAgentId": agent.Id, "recovered": recovery})); err != nil {
					return err
				}
			}
			if err := insertSchedulerEvent(txCtx, session, task, builderID, core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_CLAIMED, "LOCAL_AGENT_CLAIM", map[string]any{"agentId": agent.Id, "artifactMode": agent.ArtifactMode}); err != nil {
				return err
			}
			if _, _, err := insertOutboxEvent(txCtx, session, task.TenantId, "build.started", "build", task.Id, map[string]any{"buildId": task.Id, "appId": task.AppId, "builderId": builderID, "builderAttempt": attempt, "localAgentId": agent.Id}); err != nil {
				return err
			}
			claimed = task
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &core.LocalAgentBuildTaskResp{Base: okBase(), Task: mapBuildTask(claimed), ArtifactMode: core.HybridArtifactMode(agent.ArtifactMode)}, nil
}

func renewLocalAgentTaskLease(ctx context.Context, svcCtx *svc.ServiceContext, in *core.RenewLocalAgentTaskLeaseReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	agent, err := authenticateLocalAgent(ctx, svcCtx, in.Auth)
	if err != nil {
		return nil, err
	}
	return NewHeartbeatBuildTaskLogic(ctx, svcCtx).HeartbeatBuildTask(&core.HeartbeatBuildTaskReq{TaskId: in.TaskId, BuilderId: fmt.Sprintf("local-%d", agent.Id), BuilderAttempt: in.BuilderAttempt, LeaseSeconds: in.LeaseSeconds})
}

func reportLocalAgentBuildProgress(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ReportLocalAgentBuildProgressReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	agent, err := authenticateLocalAgent(ctx, svcCtx, in.Auth)
	if err != nil {
		return nil, err
	}
	return NewReportBuildProgressLogic(ctx, svcCtx).ReportBuildProgress(&core.ReportBuildProgressReq{TaskId: in.TaskId, BuilderId: fmt.Sprintf("local-%d", agent.Id), BuilderAttempt: in.BuilderAttempt, Status: in.Status, Progress: in.Progress, Message: in.Message})
}

func completeLocalAgentBuildTask(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CompleteLocalAgentBuildTaskReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	agent, err := authenticateLocalAgent(ctx, svcCtx, in.Auth)
	if err != nil {
		return nil, err
	}
	if err := validateHybridArtifactInput(in.ApkReference, in.ApkSha256, in.ApkSize); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.LogReference) != "" {
		if err := validateHybridArtifactInput(in.LogReference, in.LogSha256, in.LogSize); err != nil {
			return nil, err
		}
	}
	builderID := fmt.Sprintf("local-%d", agent.Id)
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var task models.TBuildTask
		if err := session.QueryRowCtx(txCtx, &task, buildTaskSelect+` WHERE id=? AND tenant_id=? AND builder_id=? AND builder_attempt=? AND status IN (?,?,?) AND lease_until>CURRENT_TIMESTAMP(3) FOR UPDATE`, in.TaskId, agent.TenantId, builderID, in.BuilderAttempt, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
			return status.Error(codes.NotFound, "build task is not owned by Local Agent or lease expired")
		}
		if err := upsertHybridArtifact(txCtx, session, agent, &task, in.BuilderAttempt, core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK, in.ApkReference, in.ApkSha256, in.ApkSize); err != nil {
			return err
		}
		if strings.TrimSpace(in.LogReference) != "" {
			if err := upsertHybridArtifact(txCtx, session, agent, &task, in.BuilderAttempt, core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILD_LOG, in.LogReference, in.LogSha256, in.LogSize); err != nil {
				return err
			}
		}
		result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status=?,apk_url=?,apk_sha256=?,apk_size=?,log_url=NULLIF(?,''),error_message=NULL,finish_time=CURRENT_TIMESTAMP(3),lease_until=NULL WHERE id=? AND builder_id=? AND builder_attempt=?`, buildStatusSuccess, in.ApkReference, in.ApkSha256, in.ApkSize, in.LogReference, task.Id, builderID, in.BuilderAttempt)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return status.Error(codes.NotFound, "build task ownership changed")
		}
		if err := releaseTaskSlot(txCtx, session, task.Id, in.BuilderAttempt, buildSlotReleased); err != nil {
			return err
		}
		if err := recordCompletedBuildUsage(txCtx, session, &task); err != nil {
			return err
		}
		if err := insertSchedulerEvent(txCtx, session, &task, builderID, core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_COMPLETED, "LOCAL_AGENT_COMPLETED", map[string]any{"agentId": agent.Id, "artifactMode": agent.ArtifactMode}); err != nil {
			return err
		}
		_, _, err = insertOutboxEvent(txCtx, session, task.TenantId, "build.succeeded", "build", task.Id, map[string]any{"buildId": task.Id, "appId": task.AppId, "apkSha256": in.ApkSha256, "apkSize": in.ApkSize, "localAgentId": agent.Id, "artifactMode": agent.ArtifactMode})
		return err
	})
	if err != nil {
		return nil, err
	}
	return workerBase(), nil
}

func failLocalAgentBuildTask(ctx context.Context, svcCtx *svc.ServiceContext, in *core.FailLocalAgentBuildTaskReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requireText(in.ErrorMessage, "error_message", 2000); err != nil {
		return nil, err
	}
	agent, err := authenticateLocalAgent(ctx, svcCtx, in.Auth)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.LogReference) != "" {
		if err := validateHybridArtifactInput(in.LogReference, in.LogSha256, in.LogSize); err != nil {
			return nil, err
		}
	}
	builderID := fmt.Sprintf("local-%d", agent.Id)
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var task models.TBuildTask
		if err := session.QueryRowCtx(txCtx, &task, buildTaskSelect+` WHERE id=? AND tenant_id=? AND builder_id=? AND builder_attempt=? AND status IN (?,?,?) AND lease_until>CURRENT_TIMESTAMP(3) FOR UPDATE`, in.TaskId, agent.TenantId, builderID, in.BuilderAttempt, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
			return status.Error(codes.NotFound, "build task is not owned by Local Agent or lease expired")
		}
		if strings.TrimSpace(in.LogReference) != "" {
			if err := upsertHybridArtifact(txCtx, session, agent, &task, in.BuilderAttempt, core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILD_LOG, in.LogReference, in.LogSha256, in.LogSize); err != nil {
				return err
			}
		}
		result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status=?,error_message=?,log_url=NULLIF(?,''),finish_time=CURRENT_TIMESTAMP(3),lease_until=NULL WHERE id=? AND builder_id=? AND builder_attempt=?`, buildStatusFailed, strings.TrimSpace(in.ErrorMessage), in.LogReference, task.Id, builderID, in.BuilderAttempt)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return status.Error(codes.NotFound, "build task ownership changed")
		}
		if err := releaseTaskSlot(txCtx, session, task.Id, in.BuilderAttempt, buildSlotReleased); err != nil {
			return err
		}
		if err := recordFailedBuildUsage(txCtx, session, &task); err != nil {
			return err
		}
		if err := insertSchedulerEvent(txCtx, session, &task, builderID, core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_FAILED, "LOCAL_AGENT_FAILED", map[string]any{"agentId": agent.Id, "error": in.ErrorMessage}); err != nil {
			return err
		}
		_, _, err = insertOutboxEvent(txCtx, session, task.TenantId, "build.failed", "build", task.Id, map[string]any{"buildId": task.Id, "appId": task.AppId, "error": in.ErrorMessage, "localAgentId": agent.Id})
		return err
	})
	if err != nil {
		return nil, err
	}
	return workerBase(), nil
}

func verifyHybridArtifact(ctx context.Context, svcCtx *svc.ServiceContext, in *core.VerifyHybridArtifactReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	agent, err := authenticateLocalAgent(ctx, svcCtx, in.Auth)
	if err != nil {
		return nil, err
	}
	if in.StorageMode != core.HybridArtifactMode(agent.ArtifactMode) {
		return nil, status.Error(codes.PermissionDenied, "artifact storage mode differs from Agent authorization")
	}
	if err := validateHybridArtifactInput(in.ObjectReference, in.Sha256, in.SizeBytes); err != nil {
		return nil, err
	}
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var task models.TBuildTask
		if err := session.QueryRowCtx(txCtx, &task, buildTaskSelect+` WHERE id=? AND tenant_id=? AND builder_id=? AND builder_attempt=? FOR UPDATE`, in.TaskId, agent.TenantId, fmt.Sprintf("local-%d", agent.Id), in.BuilderAttempt); err != nil {
			return status.Error(codes.NotFound, "build task is not owned by Local Agent")
		}
		return upsertHybridArtifact(txCtx, session, agent, &task, in.BuilderAttempt, in.ArtifactType, in.ObjectReference, in.Sha256, in.SizeBytes)
	})
	if err != nil {
		return nil, err
	}
	return workerBase(), nil
}

func authenticateLocalAgent(ctx context.Context, svcCtx *svc.ServiceContext, auth *core.LocalAgentAuth) (*models.TLocalAgent, error) {
	if auth == nil || auth.AgentId <= 0 {
		return nil, status.Error(codes.Unauthenticated, "mTLS Agent authentication is required")
	}
	if err := validateAgentClockNonce(auth.Timestamp, auth.Nonce); err != nil {
		return nil, err
	}
	serial := strings.ToLower(strings.TrimSpace(auth.CertificateSerial))
	fingerprint := strings.ToLower(strings.TrimSpace(auth.CertificateFingerprintSha256))
	if serial == "" || !sha256Pattern.MatchString(fingerprint) {
		return nil, status.Error(codes.Unauthenticated, "client certificate identity is incomplete")
	}
	var agent models.TLocalAgent
	requestAt := timeFromMillis(auth.Timestamp)
	var businessErr error
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) (txErr error) {
		// Authentication failures are expected client outcomes. Returning a gRPC
		// status directly from sqlx's transaction callback marks the database as
		// unhealthy and can open its circuit breaker after repeated bad requests.
		// No mutation occurs before any of these business errors, so commit the
		// read-only transaction and return the status after TransactCtx completes.
		defer commitQuotaBusinessError(&businessErr, &txErr)
		if err := session.QueryRowCtx(txCtx, &agent, localAgentSelect+` WHERE id=? FOR UPDATE`, auth.AgentId); err != nil {
			return status.Error(codes.Unauthenticated, "Local Agent is unknown")
		}
		if agent.Status == localAgentRevoked {
			return status.Error(codes.PermissionDenied, "Local Agent is revoked")
		}
		var cert models.TLocalAgentCertificate
		if err := session.QueryRowCtx(txCtx, &cert, localCertificateSelect+` WHERE agent_id=? AND serial_number=? AND fingerprint_sha256=? FOR UPDATE`, agent.Id, serial, fingerprint); err != nil {
			return status.Error(codes.Unauthenticated, "client certificate is unknown")
		}
		now := billingNow()
		if cert.Status != localCertificateActive || now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
			return status.Error(codes.Unauthenticated, "client certificate is revoked or expired")
		}
		if agent.LastRequestAt.Valid && !requestAt.After(agent.LastRequestAt.Time) {
			return status.Error(codes.AlreadyExists, "Agent request timestamp was already consumed")
		}
		if agent.LastNonce.Valid && agent.LastNonce.String == strings.TrimSpace(auth.Nonce) {
			return status.Error(codes.AlreadyExists, "Agent nonce was already consumed")
		}
		_, err := session.ExecCtx(txCtx, `UPDATE t_local_agent SET last_nonce=?,last_request_at=? WHERE id=?`, strings.TrimSpace(auth.Nonce), requestAt, agent.Id)
		return err
	})
	if err != nil {
		return nil, err
	}
	if businessErr != nil {
		return nil, businessErr
	}
	return &agent, nil
}

func mapLocalAgent(ctx context.Context, db sqlx.SqlConn, item *models.TLocalAgent) (*core.LocalAgent, error) {
	if item == nil {
		return nil, nil
	}
	apps := parseAppIDs(item.AllowedAppIds)
	caps, err := loadAgentCapabilities(ctx, db, item.Id)
	if err != nil {
		return nil, err
	}
	var certExpiry int64
	if item.CertificateSerial.Valid {
		var expiry time.Time
		_ = db.QueryRowCtx(ctx, &expiry, `SELECT not_after FROM t_local_agent_certificate WHERE agent_id=? AND serial_number=?`, item.Id, item.CertificateSerial.String)
		certExpiry = millis(expiry)
	}
	return &core.LocalAgent{Id: item.Id, TenantId: item.TenantId, AgentCode: item.AgentCode, AgentName: item.AgentName, PoolCode: item.PoolCode, Status: core.LocalAgentStatus(item.Status), DrainStatus: core.LocalAgentDrainStatus(item.DrainStatus), ProtocolVersion: int32(item.ProtocolVersion), AgentVersion: item.AgentVersion, ArtifactMode: core.HybridArtifactMode(item.ArtifactMode), CustomerStorageRef: stringValue(item.CustomerStorageRef), AllowedAppIds: apps, Capabilities: caps, CertificateSerial: stringValue(item.CertificateSerial), CertificateNotAfter: certExpiry, LastHeartbeatAt: timeValue(item.LastHeartbeatAt), CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime)}, nil
}

func localAgentCertificateResponse(ctx context.Context, svcCtx *svc.ServiceContext, agent *models.TLocalAgent, certificate *agentpki.Certificate) (*core.RegisterLocalAgentResp, error) {
	mapped, err := mapLocalAgent(ctx, svcCtx.DB, agent)
	if err != nil {
		return nil, err
	}
	return &core.RegisterLocalAgentResp{Base: okBase(), Data: mapped, Certificate: &core.LocalAgentCertificate{SerialNumber: certificate.Serial, FingerprintSha256: certificate.Fingerprint, CertificatePem: certificate.PEM, Status: core.LocalAgentCertificateStatus_LOCAL_AGENT_CERTIFICATE_STATUS_ACTIVE, NotBefore: millis(certificate.NotBefore), NotAfter: millis(certificate.NotAfter)}, CaCertificatePem: svcCtx.AgentPKI.CAPEM()}, nil
}

func normalizeAgentApps(ctx context.Context, db sqlx.SqlConn, tenant int64, input []int64) ([]int64, string, error) {
	set := map[int64]struct{}{}
	for _, id := range input {
		if id <= 0 {
			return nil, "", status.Error(codes.InvalidArgument, "allowed_app_ids must be positive")
		}
		set[id] = struct{}{}
	}
	ids := make([]int64, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) == 0 {
		encoded, _ := json.Marshal(ids)
		return ids, string(encoded), nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := []any{tenant}
	for _, id := range ids {
		args = append(args, id)
	}
	var count int64
	if err := db.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM t_app_application WHERE tenant_id=? AND id IN (`+placeholders+`)`, args...); err != nil {
		return nil, "", billingInternalError("validate Agent applications", err)
	}
	if count != int64(len(ids)) {
		return nil, "", status.Error(codes.PermissionDenied, "one or more allowed applications do not belong to tenant")
	}
	encoded, _ := json.Marshal(ids)
	return ids, string(encoded), nil
}
func parseAppIDs(value string) []int64 {
	var ids []int64
	_ = json.Unmarshal([]byte(value), &ids)
	return ids
}
func normalizeAgentCapabilities(input []*core.LocalAgentCapability) (map[string]string, error) {
	result := map[string]string{}
	for _, item := range input {
		if item == nil {
			continue
		}
		key := strings.TrimSpace(item.CapabilityKey)
		value := strings.TrimSpace(item.CapabilityValue)
		if !localAgentCodePattern.MatchString(key) || len(value) > 500 {
			return nil, status.Error(codes.InvalidArgument, "Agent capability is invalid")
		}
		result[key] = value
	}
	return result, nil
}
func replaceAgentCapabilities(ctx context.Context, session sqlx.Session, tenant, agent int64, caps map[string]string) error {
	if _, err := session.ExecCtx(ctx, `DELETE FROM t_local_agent_capability WHERE agent_id=?`, agent); err != nil {
		return err
	}
	keys := make([]string, 0, len(caps))
	for key := range caps {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := session.ExecCtx(ctx, `INSERT INTO t_local_agent_capability (tenant_id,agent_id,capability_key,capability_value) VALUES (?,?,?,?)`, tenant, agent, key, caps[key]); err != nil {
			return err
		}
	}
	return nil
}
func loadAgentCapabilities(ctx context.Context, db sqlx.SqlConn, agent int64) ([]*core.LocalAgentCapability, error) {
	var rows []struct {
		Key   string `db:"capability_key"`
		Value string `db:"capability_value"`
	}
	if err := db.QueryRowsCtx(ctx, &rows, `SELECT capability_key,capability_value FROM t_local_agent_capability WHERE agent_id=? ORDER BY capability_key`, agent); err != nil {
		return nil, err
	}
	result := make([]*core.LocalAgentCapability, 0, len(rows))
	for _, row := range rows {
		result = append(result, &core.LocalAgentCapability{CapabilityKey: row.Key, CapabilityValue: row.Value})
	}
	return result, nil
}
func newRegistrationToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := "afr_" + hex.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(digest[:]), nil
}
func validateAgentClockNonce(timestamp int64, nonce string) error {
	if timestamp <= 0 {
		return status.Error(codes.InvalidArgument, "timestamp is required")
	}
	if len(strings.TrimSpace(nonce)) < 16 || len(nonce) > 128 {
		return status.Error(codes.InvalidArgument, "nonce must contain 16-128 characters")
	}
	delta := billingNow().Sub(timeFromMillis(timestamp))
	if delta < 0 {
		delta = -delta
	}
	if delta > 5*time.Minute {
		return status.Error(codes.Unauthenticated, "Agent request timestamp is outside the allowed window")
	}
	return nil
}
func agentMaxConcurrency(ctx context.Context, session sqlx.Session, agent int64) int64 {
	var value string
	if err := session.QueryRowCtx(ctx, &value, `SELECT capability_value FROM t_local_agent_capability WHERE agent_id=? AND capability_key='max_concurrency'`, agent); err != nil {
		return 1
	}
	parsed, _ := strconv.ParseInt(value, 10, 64)
	if parsed < 1 || parsed > 64 {
		return 1
	}
	return parsed
}
func validateHybridArtifactInput(reference, digest string, size int64) error {
	if err := requireText(reference, "object_reference", 1000); err != nil {
		return err
	}
	if strings.ContainsAny(reference, "\r\n") || strings.Contains(reference, "@") {
		return status.Error(codes.InvalidArgument, "object_reference must not contain credentials or control characters")
	}
	if !sha256Pattern.MatchString(strings.ToLower(strings.TrimSpace(digest))) {
		return status.Error(codes.InvalidArgument, "sha256 must be 64 lowercase hexadecimal characters")
	}
	if size <= 0 {
		return status.Error(codes.InvalidArgument, "size_bytes must be greater than zero")
	}
	return nil
}
func upsertHybridArtifact(ctx context.Context, session sqlx.Session, agent *models.TLocalAgent, task *models.TBuildTask, attempt int32, artifactType core.HybridArtifactType, reference, digest string, size int64) error {
	if task.TenantId != agent.TenantId || task.BuilderAttempt != int64(attempt) {
		return status.Error(codes.PermissionDenied, "Artifact tenant or task attempt mismatch")
	}
	if artifactType < core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_SOURCE_APK || artifactType > core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_OFFLINE_TASK_PACKAGE {
		return status.Error(codes.InvalidArgument, "artifact_type is invalid")
	}
	_, err := session.ExecCtx(ctx, `INSERT INTO t_hybrid_artifact_reference
(tenant_id,agent_id,task_id,builder_attempt,artifact_type,storage_mode,object_reference,sha256,size_bytes,status,verified_at)
VALUES (?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE
object_reference=IF(object_reference=VALUES(object_reference) AND sha256=VALUES(sha256) AND size_bytes=VALUES(size_bytes),object_reference,object_reference),
status=IF(object_reference=VALUES(object_reference) AND sha256=VALUES(sha256) AND size_bytes=VALUES(size_bytes),VALUES(status),status)`, task.TenantId, agent.Id, task.Id, attempt, int64(artifactType), agent.ArtifactMode, strings.TrimSpace(reference), strings.ToLower(strings.TrimSpace(digest)), size, hybridArtifactVerified)
	if err != nil {
		return err
	}
	var matches int64
	if err := session.QueryRowCtx(ctx, &matches, `SELECT COUNT(*) FROM t_hybrid_artifact_reference WHERE tenant_id=? AND task_id=? AND builder_attempt=? AND artifact_type=? AND agent_id=? AND storage_mode=? AND object_reference=? AND sha256=? AND size_bytes=?`, task.TenantId, task.Id, attempt, int64(artifactType), agent.Id, agent.ArtifactMode, strings.TrimSpace(reference), strings.ToLower(strings.TrimSpace(digest)), size); err != nil {
		return err
	}
	if matches != 1 {
		return status.Error(codes.AlreadyExists, "Artifact idempotency key is already used with different integrity data")
	}
	return nil
}
func recordCompletedBuildUsage(ctx context.Context, session sqlx.Session, task *models.TBuildTask) error {
	_, entitlement, _, err := loadTenantBilling(ctx, session, task.TenantId, false)
	if err != nil {
		return err
	}
	if err := adjustUsageInSession(ctx, session, task.TenantId, "build.succeeded", 1, "build", task.Id, fmt.Sprintf("build-succeeded:%d", task.Id), map[string]any{"localAgent": true}); err != nil {
		return err
	}
	compute := int64(0)
	if task.StartTime.Valid {
		compute = int64(billingNow().Sub(task.StartTime.Time).Seconds())
		if compute < 0 {
			compute = 0
		}
	}
	if compute > 0 {
		if err := adjustUsageInSession(ctx, session, task.TenantId, "build.compute_seconds", compute, "build", task.Id, fmt.Sprintf("build-compute:%d", task.Id), map[string]any{"localAgent": true}); err != nil {
			return err
		}
	}
	if task.CacheHit == 1 && entitlement.ChargeCacheHit == 0 && !(task.RetryOfTaskId > 0 && entitlement.ChargeRetryBuild == 0) {
		return adjustUsageInSession(ctx, session, task.TenantId, "build.started", -1, "build", task.Id, fmt.Sprintf("build-cache-refund:%d", task.Id), map[string]any{"reason": "cache_hit_not_charged"})
	}
	return nil
}
func recordFailedBuildUsage(ctx context.Context, session sqlx.Session, task *models.TBuildTask) error {
	_, entitlement, _, err := loadTenantBilling(ctx, session, task.TenantId, false)
	if err != nil {
		return err
	}
	compute := int64(0)
	if task.StartTime.Valid {
		compute = int64(billingNow().Sub(task.StartTime.Time).Seconds())
		if compute < 0 {
			compute = 0
		}
	}
	if compute > 0 {
		if err := adjustUsageInSession(ctx, session, task.TenantId, "build.compute_seconds", compute, "build", task.Id, fmt.Sprintf("build-compute:%d", task.Id), map[string]any{"failed": true, "localAgent": true}); err != nil {
			return err
		}
	}
	if entitlement.ChargeFailedBuild == 0 && !(task.RetryOfTaskId > 0 && entitlement.ChargeRetryBuild == 0) {
		return adjustUsageInSession(ctx, session, task.TenantId, "build.started", -1, "build", task.Id, fmt.Sprintf("build-failed-refund:%d", task.Id), map[string]any{"reason": "failed_build_not_charged"})
	}
	return nil
}
