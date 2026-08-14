package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBillingPlanLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBillingPlanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBillingPlanLogic {
	return &GetBillingPlanLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询V6套餐版本。
func (l *GetBillingPlanLogic) GetBillingPlan(in *core.BillingPlanIdReq) (*core.BillingPlanResp, error) {
	return getBillingPlan(l.ctx, l.svcCtx, in)
}
