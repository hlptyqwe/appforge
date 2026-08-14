package logic

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FailBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFailBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FailBuildTaskLogic {
	return &FailBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *FailBuildTaskLogic) FailBuildTask(in *core.FailBuildTaskReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requireText(in.ErrorMessage, "error_message", 2000); err != nil {
		return nil, err
	}
	if err := validateBuilderRequest(in.BuilderId); err != nil {
		return nil, err
	}
	if err := requirePositive(in.TaskId, "task_id"); err != nil {
		return nil, err
	}
	if err := requirePositive(int64(in.BuilderAttempt), "builder_attempt"); err != nil {
		return nil, err
	}
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(txCtx context.Context, session sqlx.Session) error {
		var task models.TBuildTask
		if err := session.QueryRowCtx(txCtx, &task, buildTaskSelect+` WHERE id = ? AND builder_id = ? AND builder_attempt = ?
AND status IN (?, ?, ?) AND lease_until > CURRENT_TIMESTAMP(3) FOR UPDATE`,
			in.TaskId, in.BuilderId, in.BuilderAttempt, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
			if err == sql.ErrNoRows || err == sqlx.ErrNotFound {
				return status.Error(codes.NotFound, "build task is not owned by builder or lease has expired")
			}
			return err
		}

		var logObjectID int64
		if strings.TrimSpace(in.LogObjectKey) != "" {
			artifact := buildArtifact{
				ObjectKey: in.LogObjectKey, OriginalName: "build.log", ContentType: "text/plain; charset=utf-8",
				Size: in.LogSize, SHA256: strings.TrimSpace(in.LogSha256), ObjectType: int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG),
			}
			if err := validateBuildArtifact(task.TenantId, artifact, "build-log"); err != nil {
				return err
			}
			var err error
			logObjectID, err = insertBuildArtifact(txCtx, session, task.TenantId, task.AppId, artifact)
			if err != nil {
				return err
			}
		}

		result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status = ?, error_message = ?,
log_object_id = ?, log_url = NULLIF(?, ''), finish_time = CURRENT_TIMESTAMP, lease_until = NULL,
update_time = CURRENT_TIMESTAMP WHERE id = ? AND builder_id = ? AND builder_attempt = ? AND status IN (?, ?, ?) AND lease_until > CURRENT_TIMESTAMP`,
			buildStatusFailed, in.ErrorMessage, logObjectID, in.LogObjectKey, in.TaskId, in.BuilderId, in.BuilderAttempt,
			buildStatusBuilding, buildStatusSigning, buildStatusUploading)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			if err != nil {
				return err
			}
			return status.Error(codes.NotFound, "build task ownership changed")
		}
		if err := releaseTaskSlot(txCtx, session, task.Id, in.BuilderAttempt, buildSlotReleased); err != nil {
			return err
		}
		_, _ = session.ExecCtx(txCtx, `UPDATE t_builder_node SET running_count = GREATEST(running_count - 1, 0),
update_time = CURRENT_TIMESTAMP(3) WHERE node_code = ?`, in.BuilderId)
		task.Status = buildStatusFailed
		task.ErrorMessage = nullString(in.ErrorMessage)
		_, entitlement, _, billingErr := loadTenantBilling(txCtx, session, task.TenantId, false)
		if billingErr != nil {
			return billingErr
		}
		computeSeconds := int64(0)
		if task.StartTime.Valid {
			computeSeconds = int64(billingNow().Sub(task.StartTime.Time).Seconds())
			if computeSeconds < 0 {
				computeSeconds = 0
			}
		}
		if computeSeconds > 0 {
			if err := adjustUsageInSession(txCtx, session, task.TenantId, "build.compute_seconds", computeSeconds,
				"build", task.Id, fmt.Sprintf("build-compute:%d", task.Id), map[string]any{"failed": true}); err != nil {
				return err
			}
		}
		if entitlement.ChargeFailedBuild == 0 &&
			!(task.RetryOfTaskId > 0 && entitlement.ChargeRetryBuild == 0) {
			if err := adjustUsageInSession(txCtx, session, task.TenantId, "build.started", -1,
				"build", task.Id, fmt.Sprintf("build-failed-refund:%d", task.Id),
				map[string]any{"reason": "failed_build_not_charged"}); err != nil {
				return err
			}
		}
		if err := insertSchedulerEvent(txCtx, session, &task, in.BuilderId,
			core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_FAILED, "BUILD_FAILED",
			map[string]any{"error": in.ErrorMessage}); err != nil {
			return err
		}
		_, _, err = insertOutboxEvent(txCtx, session, task.TenantId, "build.failed", "build", task.Id,
			map[string]any{"buildId": task.Id, "appId": task.AppId, "error": in.ErrorMessage})
		return err
	})
	if err != nil {
		return nil, err
	}

	return workerBase(), nil
}
