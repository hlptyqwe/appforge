// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePlatformWebhookEndpointLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdatePlatformWebhookEndpointLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePlatformWebhookEndpointLogic {
	return &UpdatePlatformWebhookEndpointLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdatePlatformWebhookEndpointLogic) UpdatePlatformWebhookEndpoint(req *types.UpdatePlatformWebhookEndpointReq) (resp *types.PlatformWebhookEndpointResp, err error) {
	return updatePlatformWebhookEndpoint(l.ctx, l.svcCtx, req)
}
