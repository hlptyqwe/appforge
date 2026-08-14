// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/logicutil"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePlatformLocalAgentRegistrationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformLocalAgentRegistrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformLocalAgentRegistrationLogic {
	return &CreatePlatformLocalAgentRegistrationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformLocalAgentRegistrationLogic) CreatePlatformLocalAgentRegistration(req *types.CreatePlatformLocalAgentRegistrationReq) (resp *types.PlatformLocalAgentRegistrationResp, err error) {
	return logicutil.Proxy[types.PlatformLocalAgentRegistrationResp](l.ctx, req, l.svcCtx.CoreCli.CreateLocalAgentRegistration)
}
