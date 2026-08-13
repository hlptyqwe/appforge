package logic

import (
	"context"
	"database/sql"
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
channel_code, version_code, version_name, source_apk_url, build_config, status, builder_id,
builder_attempt, priority, apk_url, apk_sha256, apk_size, log_url, error_message, queued_at,
start_time, finish_time, lease_until, create_by, create_time, update_time FROM t_build_task`

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
	leaseUntil := time.Now().Add(time.Duration(seconds) * time.Second)
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := session.QueryRowCtx(txCtx, &item, buildTaskSelect+` WHERE status = ? OR (status IN (?, ?, ?) AND (lease_until IS NULL OR lease_until < ?)) ORDER BY priority DESC, id ASC LIMIT 1 FOR UPDATE`, buildStatusPending, buildStatusBuilding, buildStatusSigning, buildStatusUploading, time.Now()); err != nil {
			return err
		}
		result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status = ?, builder_id = ?, builder_attempt = builder_attempt + 1, start_time = COALESCE(start_time, ?), lease_until = ?, update_time = CURRENT_TIMESTAMP WHERE id = ?`, buildStatusBuilding, builderID, time.Now(), leaseUntil, item.Id)
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
		if !item.StartTime.Valid {
			item.StartTime = nullTime(time.Now())
		}
		item.LeaseUntil = nullTime(leaseUntil)
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

func updateTaskWithBuilder(ctx context.Context, svcCtx *svc.ServiceContext, taskID int64, builderID string, query string, args ...any) error {
	if err := validateBuilderRequest(builderID); err != nil {
		return err
	}
	if err := requirePositive(taskID, "task_id"); err != nil {
		return err
	}
	queryArgs := append(args, taskID, builderID)
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
