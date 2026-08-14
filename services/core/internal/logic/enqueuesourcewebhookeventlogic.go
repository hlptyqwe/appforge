package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type EnqueueSourceWebhookEventLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEnqueueSourceWebhookEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnqueueSourceWebhookEventLogic {
	return &EnqueueSourceWebhookEventLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 将已完成供应商签名验证的标准化事件可靠且幂等地写入队列。
func (l *EnqueueSourceWebhookEventLogic) EnqueueSourceWebhookEvent(in *core.EnqueueSourceWebhookEventReq) (*core.EnqueueSourceWebhookEventResp, error) {
	return enqueueSourceWebhookEvent(l.ctx, l.svcCtx, in)
}
