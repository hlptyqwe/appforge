package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type FailSourceWebhookEventLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFailSourceWebhookEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FailSourceWebhookEventLogic {
	return &FailSourceWebhookEventLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Worker按可重试性记录失败、指数退避或最终失败。
func (l *FailSourceWebhookEventLogic) FailSourceWebhookEvent(in *core.FailSourceWebhookEventReq) (*core.RespBase, error) {
	return failSourceWebhookEvent(l.ctx, l.svcCtx, in)
}
