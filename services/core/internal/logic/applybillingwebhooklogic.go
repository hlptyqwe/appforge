package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApplyBillingWebhookLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApplyBillingWebhookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplyBillingWebhookLogic {
	return &ApplyBillingWebhookLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 应用已验签支付事件，事件幂等且拒绝乱序覆盖新状态。
func (l *ApplyBillingWebhookLogic) ApplyBillingWebhook(in *core.ApplyBillingWebhookReq) (*core.RespBase, error) {
	return applyBillingWebhook(l.ctx, l.svcCtx, in)
}
