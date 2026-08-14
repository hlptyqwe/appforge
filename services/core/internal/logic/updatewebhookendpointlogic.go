package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateWebhookEndpointLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateWebhookEndpointLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateWebhookEndpointLogic {
	return &UpdateWebhookEndpointLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 更新Webhook订阅端点。
func (l *UpdateWebhookEndpointLogic) UpdateWebhookEndpoint(in *core.UpdateWebhookEndpointReq) (*core.WebhookEndpointResp, error) {
	return updateWebhookEndpoint(l.ctx, l.svcCtx, in)
}
