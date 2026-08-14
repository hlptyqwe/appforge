package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type BuilderNodeHeartbeatLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBuilderNodeHeartbeatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BuilderNodeHeartbeatLogic {
	return &BuilderNodeHeartbeatLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 上报V4 Builder节点心跳、容量和运行任务。
func (l *BuilderNodeHeartbeatLogic) BuilderNodeHeartbeat(in *core.BuilderNodeHeartbeatReq) (*core.BuilderNodeResp, error) {
	return heartbeatBuilderNode(l.ctx, l.svcCtx, in)
}
