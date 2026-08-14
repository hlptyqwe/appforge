// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePlatformSourceBuildTriggerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformSourceBuildTriggerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformSourceBuildTriggerLogic {
	return &CreatePlatformSourceBuildTriggerLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformSourceBuildTriggerLogic) CreatePlatformSourceBuildTrigger(req *types.CreatePlatformSourceBuildTriggerReq) (resp *types.PlatformSourceBuildTriggerSecretResp, err error) {
	return createPlatformSourceBuildTrigger(l.ctx, l.svcCtx, req)
}
