package logic

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"strings"

	"appforge/common/airgap"
	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func importAirGappedResult(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ImportAirGappedResultReq) (*core.AirGappedPackageResp, error) {
	if in == nil || strings.TrimSpace(in.PackageCode) == "" || in.ResultObjectId <= 0 || !validSHA256(in.ResultSha256) || in.ResultSizeBytes <= 0 {
		return nil, status.Error(codes.InvalidArgument, "AIR_GAPPED result object integrity is invalid")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	canonical := []byte(in.ResultManifestJson)
	manifest, err := airgap.DecodeResultManifest(canonical)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "result manifest is invalid: %v", err)
	}
	var item models.TAirGappedPackage
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := session.QueryRowCtx(txCtx, &item, airGappedPackageSelect+` WHERE package_code=? AND tenant_id=? FOR UPDATE`, strings.TrimSpace(in.PackageCode), tenant); err != nil {
			return notFoundOrInternal(err, "AIR_GAPPED package")
		}
		if item.Status == airGappedImported {
			if item.ResultObjectId == in.ResultObjectId && stringValue(item.ResultSha256) == in.ResultSha256 && item.ResultSizeBytes == in.ResultSizeBytes {
				return nil
			}
			return status.Error(codes.AlreadyExists, "AIR_GAPPED result was already imported with different integrity data")
		}
		if item.Status != airGappedExported || !billingNow().Before(item.ExpiresAt) {
			return status.Error(codes.FailedPrecondition, "AIR_GAPPED package is not exported or has expired")
		}
		if err := validateResultManifestBinding(manifest, &item); err != nil {
			return err
		}
		if _, err := validateAirGappedAgentSignature(txCtx, session, svcCtx, manifest, canonical, in); err != nil {
			return err
		}
		var task models.TBuildTask
		builderID := fmt.Sprintf("local-%d", item.AgentId)
		if err := session.QueryRowCtx(txCtx, &task, buildTaskSelect+` WHERE id=? AND tenant_id=? AND builder_id=? AND builder_attempt=? AND status IN (?,?,?) AND lease_until>CURRENT_TIMESTAMP(3) FOR UPDATE`,
			item.TaskId, tenant, builderID, item.BuilderAttempt, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
			return status.Error(codes.FailedPrecondition, "AIR_GAPPED task attempt is no longer current")
		}
		resultObject, err := lockAirGappedObject(txCtx, session, in.ResultObjectId, tenant, item.AppId,
			core.StorageObjectType_STORAGE_OBJECT_TYPE_OFFLINE_RESULT_PACKAGE, in.ResultSha256, in.ResultSizeBytes)
		if err != nil {
			return err
		}
		if err := bindAirGappedObject(txCtx, session, svcCtx, resultObject); err != nil {
			return err
		}
		if err := insertAirGappedArtifact(txCtx, session, &item, core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_OFFLINE_RESULT_PACKAGE,
			storageObjectReference(resultObject.Id), in.ResultSha256, in.ResultSizeBytes); err != nil {
			return err
		}
		apkID, logID, apkArtifact, _, err := validateAirGappedOutputs(txCtx, session, svcCtx, &item, manifest, in)
		if err != nil {
			return err
		}
		if manifest.Status == "SUCCESS" {
			result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status=?,apk_url=?,apk_object_id=?,apk_sha256=?,apk_size=?,
log_url=NULLIF(?,''),log_object_id=?,error_message=NULL,finish_time=CURRENT_TIMESTAMP(3),lease_until=NULL
WHERE id=? AND builder_id=? AND builder_attempt=?`, buildStatusSuccess, storageObjectReference(apkID), apkID,
				apkArtifact.SHA256, apkArtifact.SizeBytes, optionalStorageReference(logID), logID, task.Id, builderID, item.BuilderAttempt)
			if err != nil {
				return err
			}
			affected, _ := result.RowsAffected()
			if affected != 1 {
				return status.Error(codes.Aborted, "AIR_GAPPED task ownership changed")
			}
			if err := recordCompletedBuildUsage(txCtx, session, &task); err != nil {
				return err
			}
			if err := insertSchedulerEvent(txCtx, session, &task, builderID, core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_COMPLETED,
				"AIR_GAPPED_IMPORTED", map[string]any{"agentId": item.AgentId, "packageCode": item.PackageCode}); err != nil {
				return err
			}
			if _, _, err := insertOutboxEvent(txCtx, session, tenant, "build.succeeded", "build", task.Id,
				map[string]any{"buildId": task.Id, "appId": task.AppId, "apkSha256": apkArtifact.SHA256, "apkSize": apkArtifact.SizeBytes, "localAgentId": item.AgentId, "airGapped": true}); err != nil {
				return err
			}
		} else {
			result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status=?,error_message=?,log_url=NULLIF(?,''),
log_object_id=?,finish_time=CURRENT_TIMESTAMP(3),lease_until=NULL WHERE id=? AND builder_id=? AND builder_attempt=?`,
				buildStatusFailed, strings.TrimSpace(manifest.ErrorMessage), optionalStorageReference(logID), logID, task.Id, builderID, item.BuilderAttempt)
			if err != nil {
				return err
			}
			affected, _ := result.RowsAffected()
			if affected != 1 {
				return status.Error(codes.Aborted, "AIR_GAPPED task ownership changed")
			}
			if err := recordFailedBuildUsage(txCtx, session, &task); err != nil {
				return err
			}
			if err := insertSchedulerEvent(txCtx, session, &task, builderID, core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_FAILED,
				"AIR_GAPPED_FAILED", map[string]any{"agentId": item.AgentId, "packageCode": item.PackageCode, "error": manifest.ErrorMessage}); err != nil {
				return err
			}
			if _, _, err := insertOutboxEvent(txCtx, session, tenant, "build.failed", "build", task.Id,
				map[string]any{"buildId": task.Id, "appId": task.AppId, "error": manifest.ErrorMessage, "localAgentId": item.AgentId, "airGapped": true}); err != nil {
				return err
			}
		}
		if err := releaseTaskSlot(txCtx, session, task.Id, int32(item.BuilderAttempt), buildSlotReleased); err != nil {
			return err
		}
		_, err = session.ExecCtx(txCtx, `UPDATE t_air_gapped_package SET result_object_id=?,result_sha256=?,result_size_bytes=?,status=?,imported_at=CURRENT_TIMESTAMP(3) WHERE id=?`,
			resultObject.Id, in.ResultSha256, in.ResultSizeBytes, airGappedImported, item.Id)
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

func validateResultManifestBinding(manifest *airgap.ResultManifest, item *models.TAirGappedPackage) error {
	if manifest.PackageCode != item.PackageCode || manifest.TenantID != item.TenantId || manifest.AgentID != item.AgentId ||
		manifest.TaskID != item.TaskId || int64(manifest.BuilderAttempt) != item.BuilderAttempt ||
		strings.ToLower(manifest.AgentCertificateSerial) != strings.ToLower(item.AgentCertificateSerial) ||
		manifest.ExportPackageSHA256 != stringValue(item.ExportSha256) || manifest.BuiltAt < millis(item.IssuedAt) || manifest.BuiltAt > millis(item.ExpiresAt) {
		return status.Error(codes.PermissionDenied, "result manifest identity differs from exported package")
	}
	digest := sha256.Sum256([]byte(manifest.Nonce))
	if hex.EncodeToString(digest[:]) != item.NonceHash {
		return status.Error(codes.PermissionDenied, "result manifest nonce does not match package state")
	}
	return nil
}

func validateAirGappedAgentSignature(ctx context.Context, session sqlx.Session, svcCtx *svc.ServiceContext, manifest *airgap.ResultManifest,
	canonical []byte, in *core.ImportAirGappedResultReq) (*x509.Certificate, error) {
	builtAt := timeFromMillis(manifest.BuiltAt)
	certificate, err := svcCtx.AgentPKI.VerifyClientCertificate(in.AgentCertificatePem, manifest.TenantID, manifest.AgentID, builtAt)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "Agent result certificate is invalid: %v", err)
	}
	serial := strings.ToLower(certificate.SerialNumber.Text(16))
	fingerprint := sha256.Sum256(certificate.Raw)
	var record models.TLocalAgentCertificate
	if err := session.QueryRowCtx(ctx, &record, localCertificateSelect+` WHERE tenant_id=? AND agent_id=? AND serial_number=? FOR UPDATE`,
		manifest.TenantID, manifest.AgentID, serial); err != nil {
		return nil, status.Error(codes.Unauthenticated, "Agent result certificate is not registered")
	}
	if (record.Status != localCertificateActive && record.Status != localCertificateRotated) || record.RevokedAt.Valid ||
		record.FingerprintSha256 != hex.EncodeToString(fingerprint[:]) || builtAt.Before(record.NotBefore) || !builtAt.Before(record.NotAfter) {
		return nil, status.Error(codes.Unauthenticated, "Agent result certificate is revoked, expired or mismatched")
	}
	publicKey, ok := certificate.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "Agent result certificate key is unsupported")
	}
	if err := airgap.Verify(publicKey, canonical, airgap.Signature{Algorithm: in.SignatureAlgorithm, Value: in.SignatureBase64}); err != nil {
		return nil, status.Error(codes.Unauthenticated, "Agent result signature verification failed")
	}
	return certificate, nil
}

func validateAirGappedOutputs(ctx context.Context, session sqlx.Session, svcCtx *svc.ServiceContext, item *models.TAirGappedPackage,
	manifest *airgap.ResultManifest, in *core.ImportAirGappedResultReq) (int64, int64, *airgap.Artifact, *airgap.Artifact, error) {
	var apk, log *airgap.Artifact
	for index := range manifest.Outputs {
		artifact := &manifest.Outputs[index]
		switch artifact.Role {
		case "built_apk":
			apk = artifact
		case "build_log":
			log = artifact
		}
	}
	if manifest.Status == "SUCCESS" && (apk == nil || in.ApkObjectId <= 0) {
		return 0, 0, nil, nil, status.Error(codes.FailedPrecondition, "successful AIR_GAPPED result requires a built APK")
	}
	if manifest.Status == "FAILED" && (apk != nil || in.ApkObjectId != 0) {
		return 0, 0, nil, nil, status.Error(codes.InvalidArgument, "failed AIR_GAPPED result must not bind an APK")
	}
	if (log == nil) != (in.LogObjectId == 0) {
		return 0, 0, nil, nil, status.Error(codes.InvalidArgument, "AIR_GAPPED log object does not match signed outputs")
	}
	if apk != nil {
		object, err := lockAirGappedObject(ctx, session, in.ApkObjectId, item.TenantId, item.AppId,
			core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK, apk.SHA256, apk.SizeBytes)
		if err != nil {
			return 0, 0, nil, nil, err
		}
		if apk.ObjectID != 0 && apk.ObjectID != object.Id {
			return 0, 0, nil, nil, status.Error(codes.PermissionDenied, "signed APK object ID differs from uploaded object")
		}
		if err := bindAirGappedObject(ctx, session, svcCtx, object); err != nil {
			return 0, 0, nil, nil, err
		}
		if err := insertAirGappedArtifact(ctx, session, item, core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK,
			storageObjectReference(object.Id), apk.SHA256, apk.SizeBytes); err != nil {
			return 0, 0, nil, nil, err
		}
	}
	if log != nil {
		object, err := lockAirGappedObject(ctx, session, in.LogObjectId, item.TenantId, item.AppId,
			core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG, log.SHA256, log.SizeBytes)
		if err != nil {
			return 0, 0, nil, nil, err
		}
		if log.ObjectID != 0 && log.ObjectID != object.Id {
			return 0, 0, nil, nil, status.Error(codes.PermissionDenied, "signed log object ID differs from uploaded object")
		}
		if err := bindAirGappedObject(ctx, session, svcCtx, object); err != nil {
			return 0, 0, nil, nil, err
		}
		if err := insertAirGappedArtifact(ctx, session, item, core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILD_LOG,
			storageObjectReference(object.Id), log.SHA256, log.SizeBytes); err != nil {
			return 0, 0, nil, nil, err
		}
	}
	return in.ApkObjectId, in.LogObjectId, apk, log, nil
}
