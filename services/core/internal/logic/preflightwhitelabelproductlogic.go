package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PreflightWhiteLabelProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPreflightWhiteLabelProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PreflightWhiteLabelProductLogic {
	return &PreflightWhiteLabelProductLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 同步预检V3白标产品契约和依赖。
func (l *PreflightWhiteLabelProductLogic) PreflightWhiteLabelProduct(in *core.WhiteLabelProductIdReq) (*core.WhiteLabelProductPreflightResp, error) {
	return preflightWhiteLabelProduct(l.ctx, l.svcCtx, in)
}
