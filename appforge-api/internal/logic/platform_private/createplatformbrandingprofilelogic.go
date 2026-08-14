// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePlatformBrandingProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformBrandingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformBrandingProfileLogic {
	return &CreatePlatformBrandingProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformBrandingProfileLogic) CreatePlatformBrandingProfile(req *types.CreatePlatformBrandingProfileReq) (resp *types.PlatformBrandingProfileResp, err error) {
	return createPlatformBrandingProfile(l.ctx, l.svcCtx, req)
}
