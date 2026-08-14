package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWhiteLabelProductsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWhiteLabelProductsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWhiteLabelProductsLogic {
	return &ListWhiteLabelProductsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询V3白标产品。
func (l *ListWhiteLabelProductsLogic) ListWhiteLabelProducts(in *core.WhiteLabelProductListReq) (*core.WhiteLabelProductListResp, error) {
	return listWhiteLabelProducts(l.ctx, l.svcCtx, in)
}
