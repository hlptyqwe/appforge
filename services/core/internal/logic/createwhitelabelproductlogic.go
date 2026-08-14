package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateWhiteLabelProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateWhiteLabelProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateWhiteLabelProductLogic {
	return &CreateWhiteLabelProductLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建V3白标产品。
func (l *CreateWhiteLabelProductLogic) CreateWhiteLabelProduct(in *core.CreateWhiteLabelProductReq) (*core.WhiteLabelProductResp, error) {
	return createWhiteLabelProduct(l.ctx, l.svcCtx, in)
}
