// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RotatePlatformWebhookEndpointSecretLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRotatePlatformWebhookEndpointSecretLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RotatePlatformWebhookEndpointSecretLogic {
	return &RotatePlatformWebhookEndpointSecretLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RotatePlatformWebhookEndpointSecretLogic) RotatePlatformWebhookEndpointSecret(req *types.PlatformIdReq) (resp *types.PlatformWebhookEndpointSecretResp, err error) {
	return rotatePlatformWebhookEndpointSecret(l.ctx, l.svcCtx, req)
}
