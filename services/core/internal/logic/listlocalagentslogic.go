package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListLocalAgentsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListLocalAgentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLocalAgentsLogic {
	return &ListLocalAgentsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询V7 Local Agent。
func (l *ListLocalAgentsLogic) ListLocalAgents(in *core.LocalAgentListReq) (*core.LocalAgentListResp, error) {
	return listLocalAgents(l.ctx, l.svcCtx, in)
}
