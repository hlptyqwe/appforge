// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1ListWhiteLabelProductsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1ListWhiteLabelProductsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1ListWhiteLabelProductsLogic {
	return &OpenV1ListWhiteLabelProductsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1ListWhiteLabelProductsLogic) OpenV1ListWhiteLabelProducts(req *types.ListPlatformWhiteLabelProductsReq) (resp *types.PlatformWhiteLabelProductListResp, err error) {
	return openV1ListWhiteLabelProducts(l.ctx, l.svcCtx, req)
}
