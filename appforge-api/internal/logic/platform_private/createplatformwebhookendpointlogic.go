// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePlatformWebhookEndpointLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformWebhookEndpointLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformWebhookEndpointLogic {
	return &CreatePlatformWebhookEndpointLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformWebhookEndpointLogic) CreatePlatformWebhookEndpoint(req *types.CreatePlatformWebhookEndpointReq) (resp *types.PlatformWebhookEndpointSecretResp, err error) {
	return createPlatformWebhookEndpoint(l.ctx, l.svcCtx, req)
}
