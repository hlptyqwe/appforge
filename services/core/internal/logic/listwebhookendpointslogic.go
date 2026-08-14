package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWebhookEndpointsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWebhookEndpointsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWebhookEndpointsLogic {
	return &ListWebhookEndpointsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询Webhook订阅端点列表。
func (l *ListWebhookEndpointsLogic) ListWebhookEndpoints(in *core.WebhookEndpointListReq) (*core.WebhookEndpointListResp, error) {
	return listWebhookEndpoints(l.ctx, l.svcCtx, in)
}
