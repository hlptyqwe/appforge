package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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
	if err := updateTaskWithBuilder(l.ctx, l.svcCtx, in.TaskId, in.BuilderId,
		`UPDATE t_build_task SET status = ?, error_message = ?, log_url = NULLIF(?, ''), finish_time = CURRENT_TIMESTAMP, lease_until = NULL, update_time = CURRENT_TIMESTAMP WHERE status IN (?, ?, ?) AND id = ? AND builder_id = ?`,
		buildStatusFailed, in.ErrorMessage, in.LogUrl, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
		return nil, err
	}

	return workerBase(), nil
}
