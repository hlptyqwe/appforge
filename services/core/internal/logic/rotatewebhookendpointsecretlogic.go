package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RotateWebhookEndpointSecretLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRotateWebhookEndpointSecretLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RotateWebhookEndpointSecretLogic {
	return &RotateWebhookEndpointSecretLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 重置Webhook签名Secret并一次性返回新Secret。
func (l *RotateWebhookEndpointSecretLogic) RotateWebhookEndpointSecret(in *core.WebhookEndpointIdReq) (*core.WebhookEndpointSecretResp, error) {
	return rotateWebhookEndpointSecret(l.ctx, l.svcCtx, in)
}
