// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformWhiteLabelProductsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformWhiteLabelProductsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformWhiteLabelProductsLogic {
	return &ListPlatformWhiteLabelProductsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformWhiteLabelProductsLogic) ListPlatformWhiteLabelProducts(req *types.ListPlatformWhiteLabelProductsReq) (resp *types.PlatformWhiteLabelProductListResp, err error) {
	return listPlatformWhiteLabelProducts(l.ctx, l.svcCtx, req)
}
