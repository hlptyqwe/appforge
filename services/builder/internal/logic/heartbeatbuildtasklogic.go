package logic

import (
	"context"

	"appforge/proto/builder"
	corepb "appforge/proto/core"
	"appforge/services/builder/internal/svc"

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

func (l *HeartbeatBuildTaskLogic) HeartbeatBuildTask(in *builder.HeartbeatBuildTaskReq) (*builder.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.HeartbeatBuildTask(toCoreContext(l.ctx), &corepb.HeartbeatBuildTaskReq{TaskId: in.TaskId, BuilderId: in.BuilderId, LeaseSeconds: in.LeaseSeconds})
	if err != nil {
		return nil, err
	}
	return mapBase(resp), nil
}
