package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangeTenantSubscriptionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangeTenantSubscriptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeTenantSubscriptionLogic {
	return &ChangeTenantSubscriptionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 立即或周期末变更套餐。
func (l *ChangeTenantSubscriptionLogic) ChangeTenantSubscription(in *core.ChangeTenantSubscriptionReq) (*core.TenantBillingResp, error) {
	return changeTenantSubscription(l.ctx, l.svcCtx, in)
}
