// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformBrandingPreflightsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformBrandingPreflightsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformBrandingPreflightsLogic {
	return &ListPlatformBrandingPreflightsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformBrandingPreflightsLogic) ListPlatformBrandingPreflights(req *types.ListPlatformBrandingPreflightsReq) (resp *types.PlatformBrandingPreflightListResp, err error) {
	return listPlatformBrandingPreflights(l.ctx, l.svcCtx, req)
}
