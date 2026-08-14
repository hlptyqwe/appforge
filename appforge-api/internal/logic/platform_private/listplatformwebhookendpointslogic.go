// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformWebhookEndpointsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformWebhookEndpointsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformWebhookEndpointsLogic {
	return &ListPlatformWebhookEndpointsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformWebhookEndpointsLogic) ListPlatformWebhookEndpoints(req *types.ListPlatformWebhookEndpointsReq) (resp *types.PlatformWebhookEndpointListResp, err error) {
	return listPlatformWebhookEndpoints(l.ctx, l.svcCtx, req)
}
