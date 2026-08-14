// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformBrandingProfilesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformBrandingProfilesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformBrandingProfilesLogic {
	return &ListPlatformBrandingProfilesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformBrandingProfilesLogic) ListPlatformBrandingProfiles(req *types.ListPlatformBrandingProfilesReq) (resp *types.PlatformBrandingProfileListResp, err error) {
	return listPlatformBrandingProfiles(l.ctx, l.svcCtx, req)
}
