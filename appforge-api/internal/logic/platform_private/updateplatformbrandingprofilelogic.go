// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePlatformBrandingProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdatePlatformBrandingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePlatformBrandingProfileLogic {
	return &UpdatePlatformBrandingProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdatePlatformBrandingProfileLogic) UpdatePlatformBrandingProfile(req *types.UpdatePlatformBrandingProfileReq) (resp *types.PlatformBrandingProfileResp, err error) {
	return updatePlatformBrandingProfile(l.ctx, l.svcCtx, req)
}
