package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateLocalAgentRegistrationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateLocalAgentRegistrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateLocalAgentRegistrationLogic {
	return &CreateLocalAgentRegistrationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建V7 Local Agent并一次性返回注册码。
func (l *CreateLocalAgentRegistrationLogic) CreateLocalAgentRegistration(in *core.CreateLocalAgentRegistrationReq) (*core.LocalAgentRegistrationResp, error) {
	return createLocalAgentRegistration(l.ctx, l.svcCtx, in)
}
