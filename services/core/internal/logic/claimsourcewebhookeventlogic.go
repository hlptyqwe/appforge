package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClaimSourceWebhookEventLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClaimSourceWebhookEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimSourceWebhookEventLogic {
	return &ClaimSourceWebhookEventLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Worker使用租约原子领取待处理或超时事件。
func (l *ClaimSourceWebhookEventLogic) ClaimSourceWebhookEvent(in *core.ClaimSourceWebhookEventReq) (*core.ClaimSourceWebhookEventResp, error) {
	return claimSourceWebhookEvent(l.ctx, l.svcCtx, in)
}
