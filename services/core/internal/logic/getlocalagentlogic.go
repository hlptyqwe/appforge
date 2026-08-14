package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetLocalAgentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLocalAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLocalAgentLogic {
	return &GetLocalAgentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询V7 Local Agent详情。
func (l *GetLocalAgentLogic) GetLocalAgent(in *core.LocalAgentIdReq) (*core.LocalAgentResp, error) {
	return getLocalAgent(l.ctx, l.svcCtx, in)
}
