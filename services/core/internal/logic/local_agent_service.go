package logic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
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
	localProtocolCurrent      = int32(3)
	localProtocolMinimum      = int32(1)
	localTaskBundleProtocol   = int64(3)
	hybridArtifactVerified    = int64(2)
)

var (
	localAgentCodePattern    = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)
	sha256Pattern            = regexp.MustCompile(`^[0-9a-f]{64}$`)
	customerObjectKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
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
	if in.ArtifactMode == core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE {
		if _, _, err := parseCustomerStorageDescriptor(in.CustomerStorageRef, tenant, code); err != nil {
			return nil, err
		}
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
		return &core.LocalAgentBuildTaskResp{Base: okBase(), ArtifactMode: core.HybridArtifactMode(agent.ArtifactMode), CustomerStorageRef: stringValue(agent.CustomerStorageRef)}, nil
	}
	if agent.ProtocolVersion < localTaskBundleProtocol {
		return &core.LocalAgentBuildTaskResp{Base: okBase(), ArtifactMode: core.HybridArtifactMode(agent.ArtifactMode), CustomerStorageRef: stringValue(agent.CustomerStorageRef)}, nil
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
	return &core.LocalAgentBuildTaskResp{Base: okBase(), Task: mapBuildTask(claimed), ArtifactMode: core.HybridArtifactMode(agent.ArtifactMode), CustomerStorageRef: stringValue(agent.CustomerStorageRef)}, nil
}

func registerCustomerStorageInput(ctx context.Context, svcCtx *svc.ServiceContext, in *core.RegisterCustomerStorageInputReq) (*core.StorageObjectResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	agent, err := authenticateLocalAgent(ctx, svcCtx, in.Auth)
	if err != nil {
		return nil, err
	}
	if agent.ArtifactMode != int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE) {
		return nil, status.Error(codes.PermissionDenied, "Local Agent is not authorized for customer storage")
	}
	if in.AppId <= 0 || !containsAgentApp(parseAppIDs(agent.AllowedAppIds), in.AppId) {
		return nil, status.Error(codes.PermissionDenied, "application is not authorized for Local Agent")
	}
	switch in.ObjectType {
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK,
		core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE,
		core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO,
		core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH,
		core.StorageObjectType_STORAGE_OBJECT_TYPE_TEMPLATE_FILE:
	default:
		return nil, status.Error(codes.InvalidArgument, "customer storage input object type is not supported")
	}
	_, prefix, err := parseCustomerStorageDescriptor(stringValue(agent.CustomerStorageRef), agent.TenantId, agent.AgentCode)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Local Agent customer storage configuration is invalid")
	}
	objectKey, err := parseCustomerObjectReference(in.ObjectReference, agent.Id, prefix)
	if err != nil {
		return nil, err
	}
	digest := strings.ToLower(strings.TrimSpace(in.Sha256))
	if !sha256Pattern.MatchString(digest) {
		return nil, status.Error(codes.InvalidArgument, "sha256 must be 64 lowercase hexadecimal characters")
	}
	validation := &core.CreateStorageObjectReq{AppId: in.AppId, ObjectType: in.ObjectType, ObjectKey: objectKey,
		OriginalName: in.OriginalName, ContentType: in.ContentType, SizeBytes: in.SizeBytes}
	if err := validateStorageObjectInput(validation, agent.TenantId); err != nil {
		return nil, err
	}
	expectedKey := customerInputObjectKey(prefix, in.AppId, in.ObjectType, digest, strings.TrimSpace(in.OriginalName))
	if expectedKey == "" || objectKey != expectedKey {
		return nil, status.Error(codes.PermissionDenied, "customer input object reference does not match application, type and digest")
	}
	var item models.TStorageObject
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		err := session.QueryRowCtx(txCtx, &item, storageObjectSelect+` WHERE tenant_id=? AND object_key=? FOR UPDATE`, agent.TenantId, objectKey)
		if err == nil {
			if item.AppId != in.AppId || item.ObjectType != int64(in.ObjectType) || item.OriginalName != strings.TrimSpace(in.OriginalName) ||
				item.SizeBytes != in.SizeBytes || !item.Sha256.Valid || item.Sha256.String != digest ||
				item.StorageMode != int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE) || item.OwnerAgentId != agent.Id ||
				(item.Status != storageStatusReady && item.Status != storageStatusBound) {
				return status.Error(codes.AlreadyExists, "customer object reference is already registered with different metadata")
			}
			return nil
		}
		if err != sqlx.ErrNotFound && err != sql.ErrNoRows {
			return err
		}
		created := &models.TStorageObject{TenantId: agent.TenantId, AppId: in.AppId, ObjectType: int64(in.ObjectType),
			ObjectKey: objectKey, OriginalName: strings.TrimSpace(in.OriginalName), ContentType: strings.TrimSpace(validation.ContentType),
			SizeBytes: in.SizeBytes, Sha256: nullString(digest), Status: storageStatusReady,
			StorageMode: int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE), OwnerAgentId: agent.Id, CreateBy: 0}
		result, err := svcCtx.StorageObjectModel.WithSession(session).Insert(txCtx, created)
		if err != nil {
			return status.Errorf(codes.Internal, "register customer storage object: %v", err)
		}
		created.Id, err = result.LastInsertId()
		if err != nil {
			return status.Errorf(codes.Internal, "read customer storage object id: %v", err)
		}
		if _, err := reserveQuotaInSession(txCtx, session, agent.TenantId, "storage.bytes", in.SizeBytes,
			"storage", created.Id, storageQuotaKey(objectKey), 24*time.Hour); err != nil {
			return err
		}
		usageMetric, _ := mapUsageMetric(storageUsageMetric(created.ObjectType))
		if err := confirmQuotaInSession(txCtx, session, agent.TenantId, "storage.bytes", storageQuotaKey(objectKey),
			usageMetric, created.Id, billingUsageMetadata(map[string]any{"objectType": created.ObjectType, "customerStorage": true, "localAgentId": agent.Id})); err != nil {
			return err
		}
		return session.QueryRowCtx(txCtx, &item, storageObjectSelect+` WHERE id=?`, created.Id)
	})
	if err != nil {
		return nil, err
	}
	return &core.StorageObjectResp{Base: okBase(), Data: mapStorageObject(&item)}, nil
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
	var apkObjectID, logObjectID int64
	builderID := fmt.Sprintf("local-%d", agent.Id)
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var task models.TBuildTask
		if err := session.QueryRowCtx(txCtx, &task, buildTaskSelect+` WHERE id=? AND tenant_id=? AND builder_id=? AND builder_attempt=? AND status IN (?,?,?) AND lease_until>CURRENT_TIMESTAMP(3) FOR UPDATE`, in.TaskId, agent.TenantId, builderID, in.BuilderAttempt, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
			return status.Error(codes.NotFound, "build task is not owned by Local Agent or lease expired")
		}
		apkObjectID, err = upsertHybridArtifact(txCtx, session, svcCtx, agent, &task, in.BuilderAttempt,
			core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK, in.ApkReference, in.ApkSha256, in.ApkSize)
		if err != nil {
			return err
		}
		if strings.TrimSpace(in.LogReference) != "" {
			logObjectID, err = upsertHybridArtifact(txCtx, session, svcCtx, agent, &task, in.BuilderAttempt,
				core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILD_LOG, in.LogReference, in.LogSha256, in.LogSize)
			if err != nil {
				return err
			}
		}
		result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status=?,apk_url=?,apk_object_id=?,apk_sha256=?,apk_size=?,log_url=NULLIF(?,''),log_object_id=?,error_message=NULL,finish_time=CURRENT_TIMESTAMP(3),lease_until=NULL WHERE id=? AND builder_id=? AND builder_attempt=?`, buildStatusSuccess, in.ApkReference, apkObjectID, in.ApkSha256, in.ApkSize, in.LogReference, logObjectID, task.Id, builderID, in.BuilderAttempt)
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
	var logObjectID int64
	builderID := fmt.Sprintf("local-%d", agent.Id)
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var task models.TBuildTask
		if err := session.QueryRowCtx(txCtx, &task, buildTaskSelect+` WHERE id=? AND tenant_id=? AND builder_id=? AND builder_attempt=? AND status IN (?,?,?) AND lease_until>CURRENT_TIMESTAMP(3) FOR UPDATE`, in.TaskId, agent.TenantId, builderID, in.BuilderAttempt, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
			return status.Error(codes.NotFound, "build task is not owned by Local Agent or lease expired")
		}
		if strings.TrimSpace(in.LogReference) != "" {
			logObjectID, err = upsertHybridArtifact(txCtx, session, svcCtx, agent, &task, in.BuilderAttempt,
				core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILD_LOG, in.LogReference, in.LogSha256, in.LogSize)
			if err != nil {
				return err
			}
		}
		result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status=?,error_message=?,log_url=NULLIF(?,''),log_object_id=?,finish_time=CURRENT_TIMESTAMP(3),lease_until=NULL WHERE id=? AND builder_id=? AND builder_attempt=?`, buildStatusFailed, strings.TrimSpace(in.ErrorMessage), in.LogReference, logObjectID, task.Id, builderID, in.BuilderAttempt)
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
		_, err := upsertHybridArtifact(txCtx, session, svcCtx, agent, &task, in.BuilderAttempt, in.ArtifactType, in.ObjectReference, in.Sha256, in.SizeBytes)
		return err
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

func parseCustomerStorageDescriptor(reference string, tenantID int64, agentCode string) (string, string, error) {
	value := strings.TrimSpace(reference)
	parsed, err := url.Parse(value)
	if err != nil || strings.ToLower(parsed.Scheme) != "local-file" || parsed.User != nil ||
		(parsed.Host != "" && parsed.Host != "localhost") || parsed.RawQuery != "" || parsed.Fragment == "" ||
		strings.Contains(value, "%") || strings.ContainsAny(value, "\r\n@") {
		return "", "", status.Error(codes.InvalidArgument, "customer_storage_ref must be local-file:///name.json#registered/prefix")
	}
	secretPath := path.Clean("/" + strings.TrimSpace(parsed.Path))
	if secretPath == "/" || !strings.HasSuffix(strings.ToLower(secretPath), ".json") || strings.Contains(secretPath, "..") {
		return "", "", status.Error(codes.InvalidArgument, "customer storage Secret path is invalid")
	}
	prefix := strings.Trim(strings.TrimSpace(parsed.Fragment), "/")
	if prefix == "" || len(prefix) > 300 || path.Clean(prefix) != prefix || strings.Contains(prefix, "..") || !customerObjectKeyPattern.MatchString(prefix) {
		return "", "", status.Error(codes.InvalidArgument, "customer storage prefix is invalid")
	}
	required := fmt.Sprintf("tenants/%d/agents/%s", tenantID, strings.TrimSpace(agentCode))
	if prefix != required && !strings.HasPrefix(prefix, required+"/") {
		return "", "", status.Error(codes.PermissionDenied, "customer storage prefix must belong to the tenant and Agent code")
	}
	canonical := "local-file://" + secretPath + "#" + prefix
	if parsed.Host == "localhost" {
		canonical = "local-file://localhost" + secretPath + "#" + prefix
	}
	if value != canonical {
		return "", "", status.Error(codes.InvalidArgument, "customer_storage_ref is not canonical")
	}
	return secretPath, prefix, nil
}

func customerObjectReference(agentID int64, objectKey string) string {
	return fmt.Sprintf("customer-object://%d/%s", agentID, objectKey)
}

func parseCustomerObjectReference(reference string, agentID int64, registeredPrefix string) (string, error) {
	value := strings.TrimSpace(reference)
	parsed, err := url.Parse(value)
	if err != nil || strings.ToLower(parsed.Scheme) != "customer-object" || parsed.User != nil || parsed.RawQuery != "" ||
		parsed.Fragment != "" || strings.Contains(value, "%") || strings.ContainsAny(value, "\r\n@") || parsed.Host != strconv.FormatInt(agentID, 10) {
		return "", status.Error(codes.InvalidArgument, "customer object reference is invalid")
	}
	key := strings.TrimPrefix(parsed.Path, "/")
	if key == "" || len(key) > 480 || path.Clean(key) != key || strings.Contains(key, "..") || !customerObjectKeyPattern.MatchString(key) {
		return "", status.Error(codes.InvalidArgument, "customer object key is invalid")
	}
	if key != registeredPrefix && !strings.HasPrefix(key, registeredPrefix+"/") {
		return "", status.Error(codes.PermissionDenied, "customer object is outside the registered prefix")
	}
	if value != customerObjectReference(agentID, key) {
		return "", status.Error(codes.InvalidArgument, "customer object reference is not canonical")
	}
	return key, nil
}

func customerInputObjectKey(prefix string, appID int64, objectType core.StorageObjectType, digest, originalName string) string {
	if appID <= 0 || objectType <= core.StorageObjectType_STORAGE_OBJECT_TYPE_UNKNOWN || !sha256Pattern.MatchString(digest) {
		return ""
	}
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(originalName)))
	if extension == "" || len(extension) > 16 {
		return ""
	}
	return path.Join(prefix, "inputs", "apps", strconv.FormatInt(appID, 10), strconv.FormatInt(int64(objectType), 10), digest+extension)
}

func customerTaskObjectKey(prefix string, taskID int64, attempt int32, artifactType core.HybridArtifactType) string {
	name := ""
	switch artifactType {
	case core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK:
		name = "built.apk"
	case core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILD_LOG:
		name = "build.log"
	default:
		return ""
	}
	return path.Join(prefix, "tasks", strconv.FormatInt(taskID, 10), "attempts", strconv.FormatInt(int64(attempt), 10), name)
}

func containsAgentApp(apps []int64, appID int64) bool {
	for _, current := range apps {
		if current == appID {
			return true
		}
	}
	return false
}

func upsertHybridArtifact(ctx context.Context, session sqlx.Session, svcCtx *svc.ServiceContext, agent *models.TLocalAgent, task *models.TBuildTask, attempt int32, artifactType core.HybridArtifactType, reference, digest string, size int64) (int64, error) {
	if task.TenantId != agent.TenantId || task.BuilderAttempt != int64(attempt) {
		return 0, status.Error(codes.PermissionDenied, "Artifact tenant or task attempt mismatch")
	}
	if artifactType < core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_SOURCE_APK || artifactType > core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILD_LOG {
		return 0, status.Error(codes.InvalidArgument, "artifact_type is invalid")
	}
	objectID := int64(0)
	if agent.ArtifactMode == int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE) {
		parsedObjectID, err := parseControlPlaneStorageReference(reference)
		if err != nil {
			return 0, err
		}
		var object models.TStorageObject
		if err := session.QueryRowCtx(ctx, &object, storageObjectSelect+` WHERE id=? FOR UPDATE`, parsedObjectID); err != nil {
			return 0, notFoundOrInternal(err, "control-plane Artifact object")
		}
		if err := validateControlPlaneStorageObject(&object, task, artifactType, digest, size); err != nil {
			return 0, err
		}
		var conflicts int64
		if err := session.QueryRowCtx(ctx, &conflicts, `SELECT COUNT(*) FROM t_hybrid_artifact_reference
WHERE object_reference=? AND (tenant_id<>? OR task_id<>? OR builder_attempt<>? OR artifact_type<>?)`,
			strings.TrimSpace(reference), task.TenantId, task.Id, attempt, int64(artifactType)); err != nil {
			return 0, status.Errorf(codes.Internal, "check control-plane Artifact binding: %v", err)
		}
		if conflicts != 0 {
			return 0, status.Error(codes.AlreadyExists, "control-plane Artifact object is already bound to another task attempt")
		}
		if artifactType == core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK || artifactType == core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILD_LOG {
			if err := session.QueryRowCtx(ctx, &conflicts, `SELECT COUNT(*) FROM t_build_task
WHERE id<>? AND (apk_object_id=? OR log_object_id=?)`, task.Id, object.Id, object.Id); err != nil {
				return 0, status.Errorf(codes.Internal, "check build Artifact object binding: %v", err)
			}
			if conflicts != 0 {
				return 0, status.Error(codes.AlreadyExists, "control-plane Artifact object is already bound to another build task")
			}
		}
		if object.Status == storageStatusReady {
			object.Status = storageStatusBound
			if err := svcCtx.StorageObjectModel.WithSession(session).Update(ctx, &object); err != nil {
				return 0, status.Errorf(codes.Internal, "bind control-plane Artifact object: %v", err)
			}
		}
		objectID = object.Id
	} else if agent.ArtifactMode == int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE) {
		_, prefix, err := parseCustomerStorageDescriptor(stringValue(agent.CustomerStorageRef), agent.TenantId, agent.AgentCode)
		if err != nil {
			return 0, status.Error(codes.FailedPrecondition, "Local Agent customer storage configuration is invalid")
		}
		objectKey, err := parseCustomerObjectReference(reference, agent.Id, prefix)
		if err != nil {
			return 0, err
		}
		expectedKey := customerTaskObjectKey(prefix, task.Id, attempt, artifactType)
		if expectedKey == "" || objectKey != expectedKey {
			return 0, status.Error(codes.PermissionDenied, "customer Artifact reference does not match task, attempt and type")
		}
		objectType, originalName, contentType := customerOutputMetadata(artifactType)
		if objectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_UNKNOWN {
			return 0, status.Error(codes.InvalidArgument, "customer storage does not support this Artifact type")
		}
		var object models.TStorageObject
		findErr := session.QueryRowCtx(ctx, &object, storageObjectSelect+` WHERE tenant_id=? AND object_key=? FOR UPDATE`, task.TenantId, objectKey)
		if findErr == nil {
			if object.AppId != task.AppId || object.ObjectType != int64(objectType) || object.SizeBytes != size ||
				!object.Sha256.Valid || object.Sha256.String != strings.ToLower(strings.TrimSpace(digest)) ||
				object.StorageMode != int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE) || object.OwnerAgentId != agent.Id ||
				(object.Status != storageStatusReady && object.Status != storageStatusBound) {
				return 0, status.Error(codes.AlreadyExists, "customer Artifact object is already registered with different metadata")
			}
			objectID = object.Id
		} else if findErr != sqlx.ErrNotFound && findErr != sql.ErrNoRows {
			return 0, findErr
		} else {
			created := &models.TStorageObject{TenantId: task.TenantId, AppId: task.AppId, ObjectType: int64(objectType),
				ObjectKey: objectKey, OriginalName: originalName, ContentType: contentType, SizeBytes: size,
				Sha256: nullString(strings.ToLower(strings.TrimSpace(digest))), Status: storageStatusBound,
				StorageMode: int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE), OwnerAgentId: agent.Id, CreateBy: 0}
			result, err := svcCtx.StorageObjectModel.WithSession(session).Insert(ctx, created)
			if err != nil {
				return 0, status.Errorf(codes.Internal, "register customer Artifact object: %v", err)
			}
			objectID, err = result.LastInsertId()
			if err != nil {
				return 0, status.Errorf(codes.Internal, "read customer Artifact object id: %v", err)
			}
			if _, err := reserveQuotaInSession(ctx, session, task.TenantId, "storage.bytes", size,
				"storage", objectID, storageQuotaKey(objectKey), 24*time.Hour); err != nil {
				return 0, err
			}
			usageMetric, _ := mapUsageMetric(storageUsageMetric(int64(objectType)))
			if err := confirmQuotaInSession(ctx, session, task.TenantId, "storage.bytes", storageQuotaKey(objectKey),
				usageMetric, objectID, billingUsageMetadata(map[string]any{"objectType": objectType, "customerStorage": true, "localAgentId": agent.Id})); err != nil {
				return 0, err
			}
		}
		var conflicts int64
		if err := session.QueryRowCtx(ctx, &conflicts, `SELECT COUNT(*) FROM t_hybrid_artifact_reference
WHERE object_reference=? AND (tenant_id<>? OR agent_id<>? OR task_id<>? OR builder_attempt<>? OR artifact_type<>?)`,
			strings.TrimSpace(reference), task.TenantId, agent.Id, task.Id, attempt, int64(artifactType)); err != nil {
			return 0, status.Errorf(codes.Internal, "check customer Artifact binding: %v", err)
		}
		if conflicts != 0 {
			return 0, status.Error(codes.AlreadyExists, "customer Artifact object is already bound to another task attempt")
		}
	} else {
		return 0, status.Error(codes.FailedPrecondition, "AIR_GAPPED Artifacts must use signed package import/export RPCs")
	}
	_, err := session.ExecCtx(ctx, `INSERT INTO t_hybrid_artifact_reference
(tenant_id,agent_id,task_id,builder_attempt,artifact_type,storage_mode,object_reference,sha256,size_bytes,status,verified_at)
VALUES (?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE
object_reference=IF(object_reference=VALUES(object_reference) AND sha256=VALUES(sha256) AND size_bytes=VALUES(size_bytes),object_reference,object_reference),
status=IF(object_reference=VALUES(object_reference) AND sha256=VALUES(sha256) AND size_bytes=VALUES(size_bytes),VALUES(status),status)`, task.TenantId, agent.Id, task.Id, attempt, int64(artifactType), agent.ArtifactMode, strings.TrimSpace(reference), strings.ToLower(strings.TrimSpace(digest)), size, hybridArtifactVerified)
	if err != nil {
		return 0, err
	}
	var matches int64
	if err := session.QueryRowCtx(ctx, &matches, `SELECT COUNT(*) FROM t_hybrid_artifact_reference WHERE tenant_id=? AND task_id=? AND builder_attempt=? AND artifact_type=? AND agent_id=? AND storage_mode=? AND object_reference=? AND sha256=? AND size_bytes=?`, task.TenantId, task.Id, attempt, int64(artifactType), agent.Id, agent.ArtifactMode, strings.TrimSpace(reference), strings.ToLower(strings.TrimSpace(digest)), size); err != nil {
		return 0, err
	}
	if matches != 1 {
		return 0, status.Error(codes.AlreadyExists, "Artifact idempotency key is already used with different integrity data")
	}
	return objectID, nil
}

func customerOutputMetadata(artifactType core.HybridArtifactType) (core.StorageObjectType, string, string) {
	switch artifactType {
	case core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK:
		return core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK, "built.apk", "application/vnd.android.package-archive"
	case core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILD_LOG:
		return core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG, "build.log", "text/plain"
	default:
		return core.StorageObjectType_STORAGE_OBJECT_TYPE_UNKNOWN, "", ""
	}
}

func parseControlPlaneStorageReference(reference string) (int64, error) {
	value := strings.TrimSpace(reference)
	const prefix = "storage-object://"
	if !strings.HasPrefix(value, prefix) {
		return 0, status.Error(codes.InvalidArgument, "control-plane Artifact must use a storage-object reference")
	}
	identifier := strings.TrimPrefix(value, prefix)
	id, err := strconv.ParseInt(identifier, 10, 64)
	if err != nil || id <= 0 || value != fmt.Sprintf("%s%d", prefix, id) {
		return 0, status.Error(codes.InvalidArgument, "control-plane Artifact storage-object reference is invalid")
	}
	return id, nil
}

func validateControlPlaneStorageObject(object *models.TStorageObject, task *models.TBuildTask, artifactType core.HybridArtifactType, digest string, size int64) error {
	if object == nil || task == nil {
		return status.Error(codes.InvalidArgument, "control-plane Artifact context is required")
	}
	expectedObjectType := int64(0)
	switch artifactType {
	case core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_SOURCE_APK:
		expectedObjectType = int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK)
	case core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK:
		expectedObjectType = int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK)
	case core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILD_LOG:
		expectedObjectType = int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG)
	default:
		return status.Error(codes.InvalidArgument, "control-plane storage does not support this Artifact type")
	}
	normalizedDigest := strings.ToLower(strings.TrimSpace(digest))
	if object.TenantId != task.TenantId || object.AppId != task.AppId {
		return status.Error(codes.PermissionDenied, "control-plane Artifact tenant or application mismatch")
	}
	if object.ObjectType != expectedObjectType {
		return status.Error(codes.FailedPrecondition, "control-plane Artifact object type mismatch")
	}
	if object.StorageMode != int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE) || object.OwnerAgentId != 0 {
		return status.Error(codes.FailedPrecondition, "control-plane Artifact storage ownership mismatch")
	}
	if object.Status != storageStatusReady && object.Status != storageStatusBound {
		return status.Error(codes.FailedPrecondition, "control-plane Artifact object is not verified")
	}
	if object.SizeBytes != size || !object.Sha256.Valid || object.Sha256.String != normalizedDigest {
		return status.Error(codes.FailedPrecondition, "control-plane Artifact integrity metadata mismatch")
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
