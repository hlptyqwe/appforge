package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateWhiteLabelProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateWhiteLabelProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateWhiteLabelProductLogic {
	return &UpdateWhiteLabelProductLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 更新V3白标产品。
func (l *UpdateWhiteLabelProductLogic) UpdateWhiteLabelProduct(in *core.UpdateWhiteLabelProductReq) (*core.WhiteLabelProductResp, error) {
	return updateWhiteLabelProduct(l.ctx, l.svcCtx, in)
}
