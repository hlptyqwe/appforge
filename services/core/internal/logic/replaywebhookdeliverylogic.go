package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReplayWebhookDeliveryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReplayWebhookDeliveryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReplayWebhookDeliveryLogic {
	return &ReplayWebhookDeliveryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 重放失败或死信的Webhook投递。
func (l *ReplayWebhookDeliveryLogic) ReplayWebhookDelivery(in *core.WebhookDeliveryIdReq) (*core.WebhookDeliveryResp, error) {
	return replayWebhookDelivery(l.ctx, l.svcCtx, in)
}
