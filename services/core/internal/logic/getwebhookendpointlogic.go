package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWebhookEndpointLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWebhookEndpointLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWebhookEndpointLogic {
	return &GetWebhookEndpointLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询Webhook订阅端点。
func (l *GetWebhookEndpointLogic) GetWebhookEndpoint(in *core.WebhookEndpointIdReq) (*core.WebhookEndpointResp, error) {
	return getWebhookEndpoint(l.ctx, l.svcCtx, in)
}
