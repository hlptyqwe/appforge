// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReplayPlatformWebhookDeliveryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReplayPlatformWebhookDeliveryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReplayPlatformWebhookDeliveryLogic {
	return &ReplayPlatformWebhookDeliveryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReplayPlatformWebhookDeliveryLogic) ReplayPlatformWebhookDelivery(req *types.PlatformIdReq) (resp *types.PlatformWebhookDeliveryResp, err error) {
	return replayPlatformWebhookDelivery(l.ctx, l.svcCtx, req)
}
