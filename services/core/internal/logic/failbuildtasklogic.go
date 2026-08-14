package logic

import (
	"context"
	"database/sql"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

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
		var owner struct {
			TenantID int64 `db:"tenant_id"`
			AppID    int64 `db:"app_id"`
		}
		if err := session.QueryRowCtx(txCtx, &owner, `SELECT tenant_id, app_id FROM t_build_task
WHERE id = ? AND builder_id = ? AND builder_attempt = ? AND status IN (?, ?, ?) AND lease_until > CURRENT_TIMESTAMP FOR UPDATE`,
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
			if err := validateBuildArtifact(owner.TenantID, artifact, "build-log"); err != nil {
				return err
			}
			var err error
			logObjectID, err = insertBuildArtifact(txCtx, session, owner.TenantID, owner.AppID, artifact)
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
		return nil
	})
	if err != nil {
		return nil, err
	}

	return workerBase(), nil
}
