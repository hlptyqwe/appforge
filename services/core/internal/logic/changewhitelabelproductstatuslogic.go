package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangeWhiteLabelProductStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangeWhiteLabelProductStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeWhiteLabelProductStatusLogic {
	return &ChangeWhiteLabelProductStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 修改V3白标产品状态。
func (l *ChangeWhiteLabelProductStatusLogic) ChangeWhiteLabelProductStatus(in *core.ChangeWhiteLabelProductStatusReq) (*core.WhiteLabelProductResp, error) {
	return changeWhiteLabelProductStatus(l.ctx, l.svcCtx, in)
}
