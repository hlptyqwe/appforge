package logic

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const buildTaskSelect = `SELECT id, tenant_id, app_id, version_id, channel_id, signing_config_id,
channel_code, version_code, version_name, source_apk_object_id, source_apk_url, build_config,
branding_profile_id, branding_revision, branding_snapshot, status, builder_id,
white_label_product_id, template_revision, template_snapshot,
pool_code, cache_key, source_webhook_event_id, cache_entry_id, cache_hit, builder_attempt, priority, apk_object_id, apk_url,
apk_sha256, apk_size, log_object_id, log_url, error_message, queued_at, start_time, finish_time, lease_until,
cancel_requested_at, cancelled_at, cancel_reason, retry_of_task_id, create_by, create_time, update_time FROM t_build_task`

func validateBuilderRequest(builderID string) error {
	if strings.TrimSpace(builderID) == "" {
		return status.Error(codes.InvalidArgument, "builder_id is required")
	}
	if len(builderID) > 128 {
		return status.Error(codes.InvalidArgument, "builder_id is too long")
	}
	return nil
}

func leaseSeconds(value int32) int32 {
	if value <= 0 || value > 3600 {
		return 120
	}
	return value
}

func claimTask(ctx context.Context, svcCtx *svc.ServiceContext, builderID string, seconds int32) (*models.TBuildTask, error) {
	var item models.TBuildTask
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := session.QueryRowCtx(txCtx, &item, buildTaskSelect+` WHERE status = ? OR (status IN (?, ?, ?) AND (lease_until IS NULL OR lease_until < CURRENT_TIMESTAMP)) ORDER BY priority DESC, id ASC LIMIT 1 FOR UPDATE`, buildStatusPending, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
			return err
		}
		result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status = ?, builder_id = ?, builder_attempt = builder_attempt + 1, start_time = COALESCE(start_time, CURRENT_TIMESTAMP), lease_until = DATE_ADD(CURRENT_TIMESTAMP, INTERVAL ? SECOND), update_time = CURRENT_TIMESTAMP WHERE id = ?`, buildStatusBuilding, builderID, seconds, item.Id)
		if err != nil {
			return err
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			if affectedErr != nil {
				return affectedErr
			}
			return sql.ErrNoRows
		}
		item.Status = buildStatusBuilding
		item.BuilderId = nullString(builderID)
		item.BuilderAttempt++
		return nil
	})
	if err != nil {
		if err == sql.ErrNoRows || err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func updateTaskWithBuilder(ctx context.Context, svcCtx *svc.ServiceContext, taskID int64, builderID string, builderAttempt int32, query string, args ...any) error {
	if err := validateBuilderRequest(builderID); err != nil {
		return err
	}
	if err := requirePositive(taskID, "task_id"); err != nil {
		return err
	}
	if err := requirePositive(int64(builderAttempt), "builder_attempt"); err != nil {
		return err
	}
	queryArgs := append(args, taskID, builderID, builderAttempt)
	result, err := svcCtx.DB.ExecCtx(ctx, query, queryArgs...)
	if err != nil {
		return status.Errorf(codes.Internal, "update build task failed: %v", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return status.Errorf(codes.Internal, "read update result failed: %v", err)
	}
	if affected != 1 {
		return status.Error(codes.NotFound, "build task is not owned by builder or is already finished")
	}
	return nil
}

func workerBase() *core.RespBase {
	return &core.RespBase{Base: okBase()}
}

type buildArtifact struct {
	ObjectKey    string
	OriginalName string
	ContentType  string
	Size         int64
	SHA256       string
	ObjectType   int64
}

func validateBuildArtifact(tenantID int64, artifact buildArtifact, namespace string) error {
	key := strings.TrimSpace(artifact.ObjectKey)
	prefix := fmt.Sprintf("tenants/%d/%s/", tenantID, namespace)
	if key == "" || path.Clean(key) != key || !strings.HasPrefix(key, prefix) {
		return status.Error(codes.InvalidArgument, "invalid build artifact object key")
	}
	if artifact.Size < 0 {
		return status.Error(codes.InvalidArgument, "build artifact size must not be negative")
	}
	sha := strings.TrimSpace(artifact.SHA256)
	decoded, err := hex.DecodeString(sha)
	if err != nil || len(decoded) != 32 || sha != strings.ToLower(sha) {
		return status.Error(codes.InvalidArgument, "build artifact sha256 is invalid")
	}
	return nil
}

func insertBuildArtifact(ctx context.Context, session sqlx.Session, tenantID, appID int64, artifact buildArtifact) (int64, error) {
	result, err := session.ExecCtx(ctx, `INSERT INTO t_storage_object
(tenant_id, app_id, object_type, object_key, original_name, content_type, size_bytes, sha256, status, create_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`, tenantID, appID, artifact.ObjectType, artifact.ObjectKey,
		artifact.OriginalName, artifact.ContentType, artifact.Size, artifact.SHA256, storageStatusBound)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	key := storageQuotaKey(artifact.ObjectKey)
	if _, err := reserveQuotaInSession(ctx, session, tenantID, "storage.bytes", artifact.Size,
		"storage", id, key, 15*time.Minute); err != nil {
		return 0, err
	}
	usageMetric, _ := mapUsageMetric(storageUsageMetric(artifact.ObjectType))
	if err := confirmQuotaInSession(ctx, session, tenantID, "storage.bytes", key, usageMetric, id,
		billingUsageMetadata(map[string]any{"objectType": artifact.ObjectType, "builderArtifact": true})); err != nil {
		return 0, err
	}
	return id, nil
}
