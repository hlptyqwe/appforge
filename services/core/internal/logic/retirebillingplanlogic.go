package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetireBillingPlanLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRetireBillingPlanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetireBillingPlanLogic {
	return &RetireBillingPlanLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 退役V6套餐版本，不影响历史订阅。
func (l *RetireBillingPlanLogic) RetireBillingPlan(in *core.BillingPlanIdReq) (*core.BillingPlanResp, error) {
	return retireBillingPlan(l.ctx, l.svcCtx, in)
}
