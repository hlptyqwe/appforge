// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePlatformSourceBuildTriggerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdatePlatformSourceBuildTriggerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePlatformSourceBuildTriggerLogic {
	return &UpdatePlatformSourceBuildTriggerLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdatePlatformSourceBuildTriggerLogic) UpdatePlatformSourceBuildTrigger(req *types.UpdatePlatformSourceBuildTriggerReq) (resp *types.PlatformSourceBuildTriggerResp, err error) {
	return updatePlatformSourceBuildTrigger(l.ctx, l.svcCtx, req)
}
