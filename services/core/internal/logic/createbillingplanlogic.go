package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateBillingPlanLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateBillingPlanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateBillingPlanLogic {
	return &CreateBillingPlanLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建V6不可变套餐版本，同编码自动分配下一版本号。
func (l *CreateBillingPlanLogic) CreateBillingPlan(in *core.CreateBillingPlanReq) (*core.BillingPlanResp, error) {
	return createBillingPlan(l.ctx, l.svcCtx, in)
}
