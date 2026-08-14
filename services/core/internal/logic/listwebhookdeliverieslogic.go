package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWebhookDeliveriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWebhookDeliveriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWebhookDeliveriesLogic {
	return &ListWebhookDeliveriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询Webhook投递日志。
func (l *ListWebhookDeliveriesLogic) ListWebhookDeliveries(in *core.WebhookDeliveryListReq) (*core.WebhookDeliveryListResp, error) {
	return listWebhookDeliveries(l.ctx, l.svcCtx, in)
}
