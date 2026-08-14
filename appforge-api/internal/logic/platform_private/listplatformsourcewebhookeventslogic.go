// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformSourceWebhookEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformSourceWebhookEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformSourceWebhookEventsLogic {
	return &ListPlatformSourceWebhookEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformSourceWebhookEventsLogic) ListPlatformSourceWebhookEvents(req *types.ListPlatformSourceWebhookEventsReq) (resp *types.PlatformSourceWebhookEventListResp, err error) {
	return listPlatformSourceWebhookEvents(l.ctx, l.svcCtx, req)
}
