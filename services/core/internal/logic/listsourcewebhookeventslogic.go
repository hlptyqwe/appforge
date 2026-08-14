package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSourceWebhookEventsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSourceWebhookEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSourceWebhookEventsLogic {
	return &ListSourceWebhookEventsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询源码平台入站事件审计记录。
func (l *ListSourceWebhookEventsLogic) ListSourceWebhookEvents(in *core.SourceWebhookEventListReq) (*core.SourceWebhookEventListResp, error) {
	return listSourceWebhookEvents(l.ctx, l.svcCtx, in)
}
