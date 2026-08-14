package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWhiteLabelProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWhiteLabelProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWhiteLabelProductLogic {
	return &GetWhiteLabelProductLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询V3白标产品。
func (l *GetWhiteLabelProductLogic) GetWhiteLabelProduct(in *core.WhiteLabelProductIdReq) (*core.WhiteLabelProductResp, error) {
	return getWhiteLabelProduct(l.ctx, l.svcCtx, in)
}
