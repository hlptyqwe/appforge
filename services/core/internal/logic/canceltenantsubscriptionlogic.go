package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelTenantSubscriptionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelTenantSubscriptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelTenantSubscriptionLogic {
	return &CancelTenantSubscriptionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 立即或周期末取消订阅。
func (l *CancelTenantSubscriptionLogic) CancelTenantSubscription(in *core.CancelTenantSubscriptionReq) (*core.TenantBillingResp, error) {
	return cancelTenantSubscription(l.ctx, l.svcCtx, in)
}
