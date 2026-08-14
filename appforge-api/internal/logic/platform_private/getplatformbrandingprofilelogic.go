// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformBrandingProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformBrandingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformBrandingProfileLogic {
	return &GetPlatformBrandingProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformBrandingProfileLogic) GetPlatformBrandingProfile(req *types.PlatformIdReq) (resp *types.PlatformBrandingProfileResp, err error) {
	return getPlatformBrandingProfile(l.ctx, l.svcCtx, req.Id)
}
