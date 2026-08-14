// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePlatformBrandingPreflightLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformBrandingPreflightLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformBrandingPreflightLogic {
	return &CreatePlatformBrandingPreflightLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformBrandingPreflightLogic) CreatePlatformBrandingPreflight(req *types.CreatePlatformBrandingPreflightReq) (resp *types.PlatformBrandingPreflightResp, err error) {
	return createPlatformBrandingPreflight(l.ctx, l.svcCtx, req)
}
