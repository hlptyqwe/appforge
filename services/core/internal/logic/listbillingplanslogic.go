package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBillingPlansLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBillingPlansLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBillingPlansLogic {
	return &ListBillingPlansLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询V6套餐版本列表。
func (l *ListBillingPlansLogic) ListBillingPlans(in *core.BillingPlanListReq) (*core.BillingPlanListResp, error) {
	return listBillingPlans(l.ctx, l.svcCtx, in)
}
