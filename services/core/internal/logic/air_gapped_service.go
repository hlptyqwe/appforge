package logic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"appforge/common/agentprotocol"
	"appforge/common/airgap"
	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	airGappedPreparing = int64(core.AirGappedPackageStatus_AIR_GAPPED_PACKAGE_STATUS_PREPARING)
	airGappedExported  = int64(core.AirGappedPackageStatus_AIR_GAPPED_PACKAGE_STATUS_EXPORTED)
	airGappedImported  = int64(core.AirGappedPackageStatus_AIR_GAPPED_PACKAGE_STATUS_IMPORTED)
	airGappedExpired   = int64(core.AirGappedPackageStatus_AIR_GAPPED_PACKAGE_STATUS_EXPIRED)
	airGappedRevoked   = int64(core.AirGappedPackageStatus_AIR_GAPPED_PACKAGE_STATUS_REVOKED)
)

const airGappedPackageSelect = `SELECT id,tenant_id,app_id,package_code,agent_id,task_id,builder_attempt,
agent_certificate_serial,nonce_hash,export_object_id,export_sha256,export_size_bytes,result_object_id,
result_sha256,result_size_bytes,status,issued_at,expires_at,imported_at,create_by,create_time,update_time
FROM t_air_gapped_package`

func prepareAirGappedExport(ctx context.Context, svcCtx *svc.ServiceContext, in *core.PrepareAirGappedExportReq) (*core.PrepareAirGappedExportResp, error) {
	if in == nil || in.AgentId <= 0 || in.TaskId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "agent_id and task_id must be greater than zero")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	ttl := in.ExpiresSeconds
	if ttl == 0 {
		ttl = 24 * 60 * 60
	}
	if ttl < 300 || ttl > 7*24*60*60 {
		return nil, status.Error(codes.InvalidArgument, "expires_seconds must be between 300 and 604800")
	}
	now := billingNow()
	expires := now.Add(time.Duration(ttl) * time.Second)
	packageCode, err := newAirGappedIdentifier("agp_")
	if err != nil {
		return nil, billingInternalError("generate AIR_GAPPED package code", err)
	}
	nonce, err := newAirGappedIdentifier("agn_")
	if err != nil {
		return nil, billingInternalError("generate AIR_GAPPED nonce", err)
	}
	nonceDigest := sha256.Sum256([]byte(nonce))
	builderID := fmt.Sprintf("local-%d", in.AgentId)
	var item models.TAirGappedPackage
	var claimed models.TBuildTask
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var agent models.TLocalAgent
		if err := session.QueryRowCtx(txCtx, &agent, localAgentSelect+` WHERE id=? AND tenant_id=? FOR UPDATE`, in.AgentId, tenant); err != nil {
			return notFoundOrInternal(err, "AIR_GAPPED Local Agent")
		}
		if agent.ArtifactMode != int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_AIR_GAPPED) ||
			agent.Status == localAgentPending || agent.Status == localAgentRevoked || agent.Status == localAgentUpgradeRequired ||
			agent.DrainStatus != localAgentAccepting || !agentprotocol.CanClaimTaskBundle(int32(agent.ProtocolVersion)) {
			return status.Error(codes.FailedPrecondition, "Local Agent is not eligible for AIR_GAPPED export")
		}
		var apkCapability string
		if err := session.QueryRowCtx(txCtx, &apkCapability, `SELECT capability_value FROM t_local_agent_capability WHERE agent_id=? AND capability_key='apk'`, agent.Id); err != nil || apkCapability != "true" {
			return status.Error(codes.FailedPrecondition, "Local Agent does not advertise APK build capability")
		}
		var certificate models.TLocalAgentCertificate
		if err := session.QueryRowCtx(txCtx, &certificate, localCertificateSelect+` WHERE tenant_id=? AND agent_id=? AND serial_number=? AND status=? FOR UPDATE`,
			tenant, agent.Id, stringValue(agent.CertificateSerial), localCertificateActive); err != nil {
			return status.Error(codes.FailedPrecondition, "Local Agent has no active export certificate")
		}
		if now.Before(certificate.NotBefore) || expires.After(certificate.NotAfter) {
			return status.Error(codes.FailedPrecondition, "Local Agent certificate does not cover the offline package lifetime")
		}
		if err := session.QueryRowCtx(txCtx, &claimed, buildTaskSelect+` WHERE id=? AND tenant_id=? FOR UPDATE`, in.TaskId, tenant); err != nil {
			return notFoundOrInternal(err, "build task")
		}
		if !containsAgentApp(parseAppIDs(agent.AllowedAppIds), claimed.AppId) || claimed.PoolCode != agent.PoolCode {
			return status.Error(codes.PermissionDenied, "build task is outside Local Agent application or pool scope")
		}
		if claimed.Status != buildStatusPending && !((claimed.Status == buildStatusBuilding || claimed.Status == buildStatusSigning || claimed.Status == buildStatusUploading) &&
			(!claimed.LeaseUntil.Valid || !now.Before(claimed.LeaseUntil.Time))) {
			return status.Error(codes.FailedPrecondition, "build task is not pending or recoverable")
		}
		subscription, entitlement, _, err := loadTenantBilling(txCtx, session, tenant, true)
		if err != nil {
			return err
		}
		if !subscriptionAllowsConsumption(subscription, now) || entitlement.Status != entitlementActive || !now.Before(entitlement.ValidUntil) {
			return status.Error(codes.FailedPrecondition, "tenant subscription does not allow AIR_GAPPED build consumption")
		}
		active, err := activeSlotCount(txCtx, session, claimed.PoolCode, tenant, 0)
		if err != nil {
			return err
		}
		if entitlement.MaxBuildConcurrency >= 0 && active >= entitlement.MaxBuildConcurrency {
			return status.Error(codes.ResourceExhausted, "tenant build concurrency is exhausted")
		}
		appMax, _, _, err := schedulerPolicy(txCtx, session, tenant, claimed.AppId, claimed.PoolCode)
		if err != nil {
			return err
		}
		appActive, err := activeSlotCount(txCtx, session, claimed.PoolCode, tenant, claimed.AppId)
		if err != nil {
			return err
		}
		if appActive >= appMax {
			return status.Error(codes.ResourceExhausted, "application build concurrency is exhausted")
		}
		var agentActive int64
		if err := session.QueryRowCtx(txCtx, &agentActive, `SELECT COUNT(*) FROM t_build_slot_lease WHERE node_code=? AND status=? AND lease_until>CURRENT_TIMESTAMP(3)`, builderID, buildSlotActive); err != nil {
			return err
		}
		if agentActive >= agentMaxConcurrency(txCtx, session, agent.Id) {
			return status.Error(codes.ResourceExhausted, "Local Agent build concurrency is exhausted")
		}
		recovery := claimed.Status != buildStatusPending
		attempt := claimed.BuilderAttempt + 1
		result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status=?,builder_id=?,builder_attempt=?,
start_time=COALESCE(start_time,CURRENT_TIMESTAMP(3)),lease_until=? WHERE id=? AND builder_attempt=?`,
			buildStatusBuilding, builderID, attempt, expires, claimed.Id, claimed.BuilderAttempt)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return status.Error(codes.Aborted, "build task attempt changed during AIR_GAPPED export")
		}
		if _, err := session.ExecCtx(txCtx, `INSERT INTO t_build_slot_lease
(task_id,tenant_id,app_id,node_code,pool_code,builder_attempt,status,lease_until) VALUES (?,?,?,?,?,?,?,?)`,
			claimed.Id, tenant, claimed.AppId, builderID, claimed.PoolCode, attempt, buildSlotActive, expires); err != nil {
			return err
		}
		claimed.Status = buildStatusBuilding
		claimed.BuilderId = nullString(builderID)
		claimed.BuilderAttempt = attempt
		claimed.LeaseUntil = sql.NullTime{Time: expires, Valid: true}
		if !(claimed.RetryOfTaskId > 0 && entitlement.ChargeRetryBuild == 0) {
			if err := confirmQuotaInSession(txCtx, session, tenant, "build.count", fmt.Sprintf("build:%d", claimed.Id), "build.started", claimed.Id,
				billingUsageMetadata(map[string]any{"builderAttempt": attempt, "localAgentId": agent.Id, "airGapped": true, "recovered": recovery})); err != nil {
				return err
			}
		}
		created := &models.TAirGappedPackage{TenantId: tenant, AppId: claimed.AppId, PackageCode: packageCode,
			AgentId: agent.Id, TaskId: claimed.Id, BuilderAttempt: attempt, AgentCertificateSerial: certificate.SerialNumber,
			NonceHash: hex.EncodeToString(nonceDigest[:]), Status: airGappedPreparing, IssuedAt: now, ExpiresAt: expires, CreateBy: actorID(ctx)}
		inserted, err := svcCtx.AirGappedPackageModel.WithSession(session).Insert(txCtx, created)
		if err != nil {
			return err
		}
		created.Id, err = inserted.LastInsertId()
		if err != nil {
			return err
		}
		if err := insertSchedulerEvent(txCtx, session, &claimed, builderID, core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_CLAIMED,
			"AIR_GAPPED_EXPORT", map[string]any{"agentId": agent.Id, "packageCode": packageCode}); err != nil {
			return err
		}
		if _, _, err := insertOutboxEvent(txCtx, session, tenant, "build.started", "build", claimed.Id,
			map[string]any{"buildId": claimed.Id, "appId": claimed.AppId, "builderId": builderID, "builderAttempt": attempt, "localAgentId": agent.Id, "airGapped": true}); err != nil {
			return err
		}
		return session.QueryRowCtx(txCtx, &item, airGappedPackageSelect+` WHERE id=?`, created.Id)
	})
	if err != nil {
		return nil, err
	}
	execution, err := NewGetBuildExecutionContextLogic(ctx, svcCtx).GetBuildExecutionContext(&core.GetBuildExecutionContextReq{
		TaskId: claimed.Id, BuilderId: builderID, BuilderAttempt: int32(claimed.BuilderAttempt),
	})
	if err == nil && execution.GetData() != nil {
		err = validateAirGappedExecutionInputs(execution.Data)
	}
	if err != nil || execution.GetData() == nil {
		_, _ = svcCtx.DB.ExecCtx(ctx, `UPDATE t_air_gapped_package SET status=? WHERE id=? AND status=?`, airGappedRevoked, item.Id, airGappedPreparing)
		message := "AIR_GAPPED_BUILD_CONTEXT_UNAVAILABLE"
		_, _ = NewFailBuildTaskLogic(ctx, svcCtx).FailBuildTask(&core.FailBuildTaskReq{TaskId: claimed.Id, BuilderId: builderID,
			BuilderAttempt: int32(claimed.BuilderAttempt), ErrorMessage: message})
		if err == nil {
			err = status.Error(codes.FailedPrecondition, message)
		}
		return nil, err
	}
	return &core.PrepareAirGappedExportResp{Base: okBase(), Package: mapAirGappedPackage(&item), Execution: execution.Data, Nonce: nonce}, nil
}

func signAirGappedManifest(ctx context.Context, svcCtx *svc.ServiceContext, in *core.SignAirGappedManifestReq) (*core.SignAirGappedManifestResp, error) {
	if in == nil || strings.TrimSpace(in.PackageCode) == "" || len(in.ManifestJson) > airgap.MaxManifestBytes {
		return nil, status.Error(codes.InvalidArgument, "package_code and manifest_json are required")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	canonical := []byte(in.ManifestJson)
	manifest, err := airgap.DecodeTaskManifest(canonical)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "task manifest is invalid: %v", err)
	}
	var item models.TAirGappedPackage
	if err := svcCtx.DB.QueryRowCtx(ctx, &item, airGappedPackageSelect+` WHERE package_code=? AND tenant_id=?`, strings.TrimSpace(in.PackageCode), tenant); err != nil {
		return nil, notFoundOrInternal(err, "AIR_GAPPED package")
	}
	if item.Status != airGappedPreparing || !billingNow().Before(item.ExpiresAt) {
		return nil, status.Error(codes.FailedPrecondition, "AIR_GAPPED package is not preparing or has expired")
	}
	if err := validateTaskManifestBinding(manifest, &item); err != nil {
		return nil, err
	}
	if err := validateAirGappedBundle(manifest.Bundle, &item); err != nil {
		return nil, err
	}
	if err := validateAirGappedTaskInputs(ctx, svcCtx.DB, manifest, &item); err != nil {
		return nil, err
	}
	signature, err := svcCtx.AgentPKI.SignManifest(canonical)
	if err != nil {
		return nil, billingInternalError("sign AIR_GAPPED manifest", err)
	}
	return &core.SignAirGappedManifestResp{Base: okBase(), Algorithm: signature.Algorithm,
		SignatureBase64: signature.Value, SignerCertificatePem: svcCtx.AgentPKI.CAPEM()}, nil
}

func finalizeAirGappedExport(ctx context.Context, svcCtx *svc.ServiceContext, in *core.FinalizeAirGappedExportReq) (*core.AirGappedPackageResp, error) {
	if in == nil || strings.TrimSpace(in.PackageCode) == "" || in.ExportObjectId <= 0 || !validSHA256(in.ExportSha256) || in.ExportSizeBytes <= 0 {
		return nil, status.Error(codes.InvalidArgument, "AIR_GAPPED export object integrity is invalid")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	var item models.TAirGappedPackage
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := session.QueryRowCtx(txCtx, &item, airGappedPackageSelect+` WHERE package_code=? AND tenant_id=? FOR UPDATE`, strings.TrimSpace(in.PackageCode), tenant); err != nil {
			return notFoundOrInternal(err, "AIR_GAPPED package")
		}
		if item.Status == airGappedExported && item.ExportObjectId == in.ExportObjectId && stringValue(item.ExportSha256) == in.ExportSha256 && item.ExportSizeBytes == in.ExportSizeBytes {
			return nil
		}
		if item.Status != airGappedPreparing || !billingNow().Before(item.ExpiresAt) {
			return status.Error(codes.FailedPrecondition, "AIR_GAPPED package cannot be finalized")
		}
		object, err := lockAirGappedObject(txCtx, session, in.ExportObjectId, tenant, item.AppId,
			core.StorageObjectType_STORAGE_OBJECT_TYPE_OFFLINE_TASK_PACKAGE, in.ExportSha256, in.ExportSizeBytes)
		if err != nil {
			return err
		}
		if err := bindAirGappedObject(txCtx, session, svcCtx, object); err != nil {
			return err
		}
		if err := insertAirGappedArtifact(txCtx, session, &item, core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_OFFLINE_TASK_PACKAGE,
			storageObjectReference(object.Id), in.ExportSha256, in.ExportSizeBytes); err != nil {
			return err
		}
		_, err = session.ExecCtx(txCtx, `UPDATE t_air_gapped_package SET export_object_id=?,export_sha256=?,export_size_bytes=?,status=? WHERE id=?`,
			object.Id, in.ExportSha256, in.ExportSizeBytes, airGappedExported, item.Id)
		if err != nil {
			return err
		}
		return session.QueryRowCtx(txCtx, &item, airGappedPackageSelect+` WHERE id=?`, item.Id)
	})
	if err != nil {
		return nil, err
	}
	return &core.AirGappedPackageResp{Base: okBase(), Data: mapAirGappedPackage(&item)}, nil
}

func getAirGappedPackage(ctx context.Context, svcCtx *svc.ServiceContext, in *core.AirGappedPackageReq) (*core.AirGappedPackageResp, error) {
	if in == nil || strings.TrimSpace(in.PackageCode) == "" {
		return nil, status.Error(codes.InvalidArgument, "package_code is required")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	var item models.TAirGappedPackage
	if err := svcCtx.DB.QueryRowCtx(ctx, &item, airGappedPackageSelect+` WHERE package_code=? AND tenant_id=?`, strings.TrimSpace(in.PackageCode), tenant); err != nil {
		return nil, notFoundOrInternal(err, "AIR_GAPPED package")
	}
	if (item.Status == airGappedPreparing || item.Status == airGappedExported) && !billingNow().Before(item.ExpiresAt) {
		item.Status = airGappedExpired
		_, _ = svcCtx.DB.ExecCtx(ctx, `UPDATE t_air_gapped_package SET status=? WHERE id=? AND status IN (?,?)`, airGappedExpired, item.Id, airGappedPreparing, airGappedExported)
	}
	return &core.AirGappedPackageResp{Base: okBase(), Data: mapAirGappedPackage(&item)}, nil
}

func abortAirGappedExport(ctx context.Context, svcCtx *svc.ServiceContext, in *core.AbortAirGappedExportReq) (*core.AirGappedPackageResp, error) {
	if in == nil || strings.TrimSpace(in.PackageCode) == "" {
		return nil, status.Error(codes.InvalidArgument, "package_code is required")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "AIR_GAPPED_EXPORT_ABORTED"
	}
	if len(reason) > 500 {
		return nil, status.Error(codes.InvalidArgument, "reason is too long")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	var item models.TAirGappedPackage
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := session.QueryRowCtx(txCtx, &item, airGappedPackageSelect+` WHERE package_code=? AND tenant_id=? FOR UPDATE`, strings.TrimSpace(in.PackageCode), tenant); err != nil {
			return notFoundOrInternal(err, "AIR_GAPPED package")
		}
		if item.Status == airGappedRevoked {
			return nil
		}
		if item.Status == airGappedImported {
			return status.Error(codes.FailedPrecondition, "imported AIR_GAPPED package cannot be revoked")
		}
		builderID := fmt.Sprintf("local-%d", item.AgentId)
		var task models.TBuildTask
		taskErr := session.QueryRowCtx(txCtx, &task, buildTaskSelect+` WHERE id=? AND tenant_id=? AND builder_id=? AND builder_attempt=? AND status IN (?,?,?) FOR UPDATE`,
			item.TaskId, tenant, builderID, item.BuilderAttempt, buildStatusBuilding, buildStatusSigning, buildStatusUploading)
		if taskErr == nil {
			result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status=?,error_message=?,finish_time=CURRENT_TIMESTAMP(3),lease_until=NULL
WHERE id=? AND builder_id=? AND builder_attempt=? AND status IN (?,?,?)`, buildStatusFailed, reason, task.Id, builderID, item.BuilderAttempt,
				buildStatusBuilding, buildStatusSigning, buildStatusUploading)
			if err != nil {
				return err
			}
			affected, _ := result.RowsAffected()
			if affected != 1 {
				return status.Error(codes.Aborted, "AIR_GAPPED task ownership changed")
			}
			if err := releaseTaskSlot(txCtx, session, task.Id, int32(item.BuilderAttempt), buildSlotCancelled); err != nil {
				return err
			}
			if err := recordFailedBuildUsage(txCtx, session, &task); err != nil {
				return err
			}
			if err := insertSchedulerEvent(txCtx, session, &task, builderID, core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_FAILED,
				"AIR_GAPPED_ABORTED", map[string]any{"agentId": item.AgentId, "packageCode": item.PackageCode, "reason": reason}); err != nil {
				return err
			}
			if _, _, err := insertOutboxEvent(txCtx, session, tenant, "build.failed", "build", task.Id,
				map[string]any{"buildId": task.Id, "appId": task.AppId, "error": reason, "localAgentId": item.AgentId, "airGapped": true}); err != nil {
				return err
			}
		} else if taskErr != sql.ErrNoRows && taskErr != sqlx.ErrNotFound {
			return taskErr
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_air_gapped_package SET status=? WHERE id=?`, airGappedRevoked, item.Id); err != nil {
			return err
		}
		return session.QueryRowCtx(txCtx, &item, airGappedPackageSelect+` WHERE id=?`, item.Id)
	})
	if err != nil {
		return nil, err
	}
	return &core.AirGappedPackageResp{Base: okBase(), Data: mapAirGappedPackage(&item)}, nil
}

func validateTaskManifestBinding(manifest *airgap.TaskManifest, item *models.TAirGappedPackage) error {
	if manifest.PackageCode != item.PackageCode || manifest.TenantID != item.TenantId || manifest.AgentID != item.AgentId ||
		manifest.TaskID != item.TaskId || int64(manifest.BuilderAttempt) != item.BuilderAttempt ||
		strings.ToLower(manifest.AgentCertificateSerial) != strings.ToLower(item.AgentCertificateSerial) ||
		manifest.IssuedAt != millis(item.IssuedAt) || manifest.ExpiresAt != millis(item.ExpiresAt) {
		return status.Error(codes.PermissionDenied, "task manifest identity differs from AIR_GAPPED package state")
	}
	digest := sha256.Sum256([]byte(manifest.Nonce))
	if hex.EncodeToString(digest[:]) != item.NonceHash {
		return status.Error(codes.PermissionDenied, "task manifest nonce does not match package state")
	}
	return nil
}

func validateAirGappedBundle(raw json.RawMessage, item *models.TAirGappedPackage) error {
	var bundle struct {
		SchemaVersion    int32  `json:"schema_version"`
		SigningSecretRef string `json:"signing_secret_ref"`
		BlockedReason    string `json:"blocked_reason"`
		Task             struct {
			ID             int64 `json:"id"`
			TenantID       int64 `json:"tenant_id"`
			AppID          int64 `json:"app_id"`
			BuilderAttempt int32 `json:"builder_attempt"`
		} `json:"task"`
	}
	if err := json.Unmarshal(raw, &bundle); err != nil || bundle.SchemaVersion != agentprotocol.Current || bundle.Task.ID != item.TaskId ||
		bundle.Task.TenantID != item.TenantId || bundle.Task.AppID != item.AppId || int64(bundle.Task.BuilderAttempt) != item.BuilderAttempt ||
		bundle.BlockedReason != "" || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(bundle.SigningSecretRef)), "local-file://") {
		return status.Error(codes.FailedPrecondition, "AIR_GAPPED task bundle identity or local Secret reference is invalid")
	}
	encoded := string(raw)
	for _, forbidden := range []string{"keystore_password_ciphertext", "key_password_ciphertext", "presigned", "access_key", "secret_key", "sb1."} {
		if strings.Contains(strings.ToLower(encoded), forbidden) {
			return status.Error(codes.FailedPrecondition, "AIR_GAPPED task bundle contains forbidden secret or transfer metadata")
		}
	}
	return nil
}

func validateAirGappedTaskInputs(ctx context.Context, db sqlx.SqlConn, manifest *airgap.TaskManifest, item *models.TAirGappedPackage) error {
	for _, artifact := range manifest.Inputs {
		expectedType := int64(0)
		switch artifact.Role {
		case "source_apk":
			expectedType = int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK)
		case "keystore":
			expectedType = int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE)
		case "brand_logo":
			expectedType = int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO)
		case "brand_splash":
			expectedType = int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH)
		case "template_file":
			expectedType = int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_TEMPLATE_FILE)
		default:
			return status.Errorf(codes.InvalidArgument, "AIR_GAPPED input role %q is unsupported", artifact.Role)
		}
		var object models.TStorageObject
		if err := db.QueryRowCtx(ctx, &object, storageObjectSelect+` WHERE id=?`, artifact.ObjectID); err != nil {
			return notFoundOrInternal(err, "AIR_GAPPED input object")
		}
		if object.TenantId != item.TenantId || object.AppId != item.AppId || object.ObjectType != expectedType ||
			object.StorageMode != int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE) || object.OwnerAgentId != 0 ||
			(object.Status != storageStatusReady && object.Status != storageStatusBound) || object.SizeBytes != artifact.SizeBytes ||
			!object.Sha256.Valid || object.Sha256.String != artifact.SHA256 || artifact.ObjectType != int32(object.ObjectType) ||
			artifact.OriginalName != object.OriginalName || artifact.ContentType != object.ContentType {
			return status.Errorf(codes.FailedPrecondition, "AIR_GAPPED input %q metadata or ownership mismatch", artifact.Role)
		}
	}
	return nil
}

func validateAirGappedExecutionInputs(execution *core.BuildExecutionContext) error {
	if execution == nil || execution.Task == nil {
		return status.Error(codes.FailedPrecondition, "AIR_GAPPED execution context is missing")
	}
	inputs := []*core.StorageObject{execution.SourceApk, execution.Keystore, execution.BrandLogo, execution.BrandSplash}
	inputs = append(inputs, execution.TemplateFiles...)
	for _, object := range inputs {
		if object == nil {
			continue
		}
		if object.StorageMode != core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE || object.OwnerAgentId != 0 {
			return status.Error(codes.FailedPrecondition, "AIR_GAPPED inputs must be owned by control-plane private storage")
		}
	}
	return nil
}

func lockAirGappedObject(ctx context.Context, session sqlx.Session, id, tenant, app int64, objectType core.StorageObjectType,
	digest string, size int64) (*models.TStorageObject, error) {
	var object models.TStorageObject
	if err := session.QueryRowCtx(ctx, &object, storageObjectSelect+` WHERE id=? FOR UPDATE`, id); err != nil {
		return nil, notFoundOrInternal(err, "AIR_GAPPED storage object")
	}
	if object.TenantId != tenant || object.AppId != app || object.ObjectType != int64(objectType) ||
		object.StorageMode != int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE) || object.OwnerAgentId != 0 ||
		(object.Status != storageStatusReady && object.Status != storageStatusBound) || object.SizeBytes != size ||
		!object.Sha256.Valid || object.Sha256.String != strings.ToLower(strings.TrimSpace(digest)) {
		return nil, status.Error(codes.FailedPrecondition, "AIR_GAPPED storage object metadata, integrity or ownership mismatch")
	}
	return &object, nil
}

func bindAirGappedObject(ctx context.Context, session sqlx.Session, svcCtx *svc.ServiceContext, object *models.TStorageObject) error {
	if object.Status == storageStatusBound {
		return nil
	}
	object.Status = storageStatusBound
	if err := svcCtx.StorageObjectModel.WithSession(session).Update(ctx, object); err != nil {
		return status.Errorf(codes.Internal, "bind AIR_GAPPED storage object: %v", err)
	}
	return nil
}

func insertAirGappedArtifact(ctx context.Context, session sqlx.Session, item *models.TAirGappedPackage, artifactType core.HybridArtifactType,
	reference, digest string, size int64) error {
	_, err := session.ExecCtx(ctx, `INSERT INTO t_hybrid_artifact_reference
(tenant_id,agent_id,task_id,builder_attempt,artifact_type,storage_mode,object_reference,sha256,size_bytes,status,verified_at)
VALUES (?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE
object_reference=IF(object_reference=VALUES(object_reference) AND sha256=VALUES(sha256) AND size_bytes=VALUES(size_bytes),object_reference,object_reference),
status=IF(object_reference=VALUES(object_reference) AND sha256=VALUES(sha256) AND size_bytes=VALUES(size_bytes),VALUES(status),status)`,
		item.TenantId, item.AgentId, item.TaskId, item.BuilderAttempt, int64(artifactType),
		int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_AIR_GAPPED), reference, digest, size, hybridArtifactVerified)
	if err != nil {
		return err
	}
	var matches int64
	if err := session.QueryRowCtx(ctx, &matches, `SELECT COUNT(*) FROM t_hybrid_artifact_reference
WHERE tenant_id=? AND agent_id=? AND task_id=? AND builder_attempt=? AND artifact_type=? AND storage_mode=? AND object_reference=? AND sha256=? AND size_bytes=?`,
		item.TenantId, item.AgentId, item.TaskId, item.BuilderAttempt, int64(artifactType),
		int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_AIR_GAPPED), reference, digest, size); err != nil {
		return err
	}
	if matches != 1 {
		return status.Error(codes.AlreadyExists, "AIR_GAPPED Artifact was already bound with different integrity data")
	}
	return nil
}

func mapAirGappedPackage(item *models.TAirGappedPackage) *core.AirGappedPackage {
	if item == nil {
		return nil
	}
	return &core.AirGappedPackage{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, PackageCode: item.PackageCode,
		AgentId: item.AgentId, TaskId: item.TaskId, BuilderAttempt: int32(item.BuilderAttempt), AgentCertificateSerial: item.AgentCertificateSerial,
		Status: core.AirGappedPackageStatus(item.Status), ExportObjectId: item.ExportObjectId, ExportSha256: stringValue(item.ExportSha256),
		ExportSizeBytes: item.ExportSizeBytes, ResultObjectId: item.ResultObjectId, ResultSha256: stringValue(item.ResultSha256),
		ResultSizeBytes: item.ResultSizeBytes, IssuedAt: millis(item.IssuedAt), ExpiresAt: millis(item.ExpiresAt),
		ImportedAt: timeValue(item.ImportedAt), CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime)}
}

func newAirGappedIdentifier(prefix string) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw), nil
}

func storageObjectReference(id int64) string { return fmt.Sprintf("storage-object://%d", id) }

func optionalStorageReference(id int64) string {
	if id <= 0 {
		return ""
	}
	return storageObjectReference(id)
}
