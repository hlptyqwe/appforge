package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertManualSubscriptionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertManualSubscriptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertManualSubscriptionLogic {
	return &UpsertManualSubscriptionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建或更新人工合同订阅并原子刷新权益。
func (l *UpsertManualSubscriptionLogic) UpsertManualSubscription(in *core.UpsertManualSubscriptionReq) (*core.TenantBillingResp, error) {
	return upsertManualSubscription(l.ctx, l.svcCtx, in)
}
