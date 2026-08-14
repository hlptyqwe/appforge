package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type HeartbeatLocalAgentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHeartbeatLocalAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HeartbeatLocalAgentLogic {
	return &HeartbeatLocalAgentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Agent通过mTLS出站连接上报心跳和预定义能力。
func (l *HeartbeatLocalAgentLogic) HeartbeatLocalAgent(in *core.HeartbeatLocalAgentReq) (*core.LocalAgentResp, error) {
	return heartbeatLocalAgent(l.ctx, l.svcCtx, in)
}
