// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformWebhookDeliveriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformWebhookDeliveriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformWebhookDeliveriesLogic {
	return &ListPlatformWebhookDeliveriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformWebhookDeliveriesLogic) ListPlatformWebhookDeliveries(req *types.ListPlatformWebhookDeliveriesReq) (resp *types.PlatformWebhookDeliveryListResp, err error) {
	return listPlatformWebhookDeliveries(l.ctx, l.svcCtx, req)
}
