package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateWebhookEndpointLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateWebhookEndpointLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateWebhookEndpointLogic {
	return &CreateWebhookEndpointLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建Webhook订阅端点并一次性返回签名Secret。
func (l *CreateWebhookEndpointLogic) CreateWebhookEndpoint(in *core.CreateWebhookEndpointReq) (*core.WebhookEndpointSecretResp, error) {
	return createWebhookEndpoint(l.ctx, l.svcCtx, in)
}
