// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformWebhookEndpointLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformWebhookEndpointLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformWebhookEndpointLogic {
	return &GetPlatformWebhookEndpointLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformWebhookEndpointLogic) GetPlatformWebhookEndpoint(req *types.PlatformIdReq) (resp *types.PlatformWebhookEndpointResp, err error) {
	return getPlatformWebhookEndpoint(l.ctx, l.svcCtx, req)
}
