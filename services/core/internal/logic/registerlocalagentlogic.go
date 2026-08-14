package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLocalAgentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterLocalAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLocalAgentLogic {
	return &RegisterLocalAgentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Agent使用一次性注册码和本地CSR完成首次注册。
func (l *RegisterLocalAgentLogic) RegisterLocalAgent(in *core.RegisterLocalAgentReq) (*core.RegisterLocalAgentResp, error) {
	return registerLocalAgent(l.ctx, l.svcCtx, in)
}
