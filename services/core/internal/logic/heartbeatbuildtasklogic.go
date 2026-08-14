package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HeartbeatBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHeartbeatBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HeartbeatBuildTaskLogic {
	return &HeartbeatBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HeartbeatBuildTaskLogic) HeartbeatBuildTask(in *core.HeartbeatBuildTaskReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := updateTaskWithBuilder(l.ctx, l.svcCtx, in.TaskId, in.BuilderId, in.BuilderAttempt,
		`UPDATE t_build_task SET lease_until = DATE_ADD(CURRENT_TIMESTAMP, INTERVAL ? SECOND), update_time = CURRENT_TIMESTAMP WHERE status IN (?, ?, ?) AND id = ? AND builder_id = ? AND builder_attempt = ? AND lease_until > CURRENT_TIMESTAMP`,
		leaseSeconds(in.LeaseSeconds), buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
		return nil, err
	}

	return workerBase(), nil
}
