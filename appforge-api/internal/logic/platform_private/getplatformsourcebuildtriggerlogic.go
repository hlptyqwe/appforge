// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformSourceBuildTriggerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformSourceBuildTriggerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformSourceBuildTriggerLogic {
	return &GetPlatformSourceBuildTriggerLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformSourceBuildTriggerLogic) GetPlatformSourceBuildTrigger(req *types.PlatformIdReq) (resp *types.PlatformSourceBuildTriggerResp, err error) {
	return getPlatformSourceBuildTrigger(l.ctx, l.svcCtx, req.Id)
}
