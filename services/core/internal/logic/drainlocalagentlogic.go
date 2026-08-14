package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DrainLocalAgentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDrainLocalAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DrainLocalAgentLogic {
	return &DrainLocalAgentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 管理员远程设置Agent Drain状态。
func (l *DrainLocalAgentLogic) DrainLocalAgent(in *core.DrainLocalAgentReq) (*core.LocalAgentResp, error) {
	return drainLocalAgent(l.ctx, l.svcCtx, in)
}
