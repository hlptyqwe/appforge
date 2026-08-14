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

type CompleteBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCompleteBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteBuildTaskLogic {
	return &CompleteBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CompleteBuildTaskLogic) CompleteBuildTask(in *core.CompleteBuildTaskReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requireText(in.ApkObjectKey, "apk_object_key", 500); err != nil {
		return nil, err
	}
	if sha := strings.TrimSpace(in.ApkSha256); len(sha) != 64 {
		return nil, status.Error(codes.InvalidArgument, "apk_sha256 must be 64 characters")
	}
	if in.ApkSize <= 0 {
		return nil, status.Error(codes.InvalidArgument, "apk_size must be greater than zero")
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

		apk := buildArtifact{
			ObjectKey: in.ApkObjectKey, OriginalName: "channel.apk",
			ContentType: "application/vnd.android.package-archive", Size: in.ApkSize,
			SHA256: strings.TrimSpace(in.ApkSha256), ObjectType: int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK),
		}
		if err := validateBuildArtifact(task.TenantId, apk, "build-apk"); err != nil {
			return err
		}
		apkObjectID, err := insertBuildArtifact(txCtx, session, task.TenantId, task.AppId, apk)
		if err != nil {
			return err
		}

		var logObjectID int64
		if strings.TrimSpace(in.LogObjectKey) != "" {
			logArtifact := buildArtifact{
				ObjectKey: in.LogObjectKey, OriginalName: "build.log", ContentType: "text/plain; charset=utf-8",
				Size: in.LogSize, SHA256: strings.TrimSpace(in.LogSha256), ObjectType: int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG),
			}
			if err := validateBuildArtifact(task.TenantId, logArtifact, "build-log"); err != nil {
				return err
			}
			logObjectID, err = insertBuildArtifact(txCtx, session, task.TenantId, task.AppId, logArtifact)
			if err != nil {
				return err
			}
		}

		result, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status = ?, apk_object_id = ?, apk_url = ?,
apk_sha256 = ?, apk_size = ?, log_object_id = ?, log_url = NULLIF(?, ''), error_message = NULL,
finish_time = CURRENT_TIMESTAMP, lease_until = NULL, update_time = CURRENT_TIMESTAMP
WHERE id = ? AND builder_id = ? AND builder_attempt = ? AND status IN (?, ?, ?) AND lease_until > CURRENT_TIMESTAMP`,
			buildStatusSuccess, apkObjectID, in.ApkObjectKey, in.ApkSha256, in.ApkSize,
			logObjectID, in.LogObjectKey, in.TaskId, in.BuilderId, in.BuilderAttempt,
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
		task.Status = buildStatusSuccess
		task.ApkObjectId = apkObjectID
		task.ApkUrl = nullString(in.ApkObjectKey)
		task.ApkSha256 = nullString(in.ApkSha256)
		task.ApkSize = in.ApkSize
		_, entitlement, _, billingErr := loadTenantBilling(txCtx, session, task.TenantId, false)
		if billingErr != nil {
			return billingErr
		}
		if err := adjustUsageInSession(txCtx, session, task.TenantId, "build.succeeded", 1,
			"build", task.Id, fmt.Sprintf("build-succeeded:%d", task.Id),
			map[string]any{"cacheHit": task.CacheHit == 1, "retry": task.RetryOfTaskId > 0}); err != nil {
			return err
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
				"build", task.Id, fmt.Sprintf("build-compute:%d", task.Id), nil); err != nil {
				return err
			}
		}
		if task.CacheHit == 1 && entitlement.ChargeCacheHit == 0 &&
			!(task.RetryOfTaskId > 0 && entitlement.ChargeRetryBuild == 0) {
			if err := adjustUsageInSession(txCtx, session, task.TenantId, "build.started", -1,
				"build", task.Id, fmt.Sprintf("build-cache-refund:%d", task.Id),
				map[string]any{"reason": "cache_hit_not_charged"}); err != nil {
				return err
			}
		}
		if err := insertSchedulerEvent(txCtx, session, &task, in.BuilderId,
			core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_COMPLETED, "BUILD_COMPLETED",
			map[string]any{"apkObjectId": apkObjectID, "cacheHit": task.CacheHit == 1}); err != nil {
			return err
		}
		_, _, err = insertOutboxEvent(txCtx, session, task.TenantId, "build.succeeded", "build", task.Id,
			map[string]any{"buildId": task.Id, "appId": task.AppId, "artifactObjectId": apkObjectID,
				"apkSha256": in.ApkSha256, "apkSize": in.ApkSize, "cacheHit": task.CacheHit == 1})
		return err
	})
	if err != nil {
		return nil, err
	}

	return workerBase(), nil
}
