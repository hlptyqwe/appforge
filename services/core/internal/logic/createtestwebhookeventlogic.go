package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateTestWebhookEventLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTestWebhookEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTestWebhookEventLogic {
	return &CreateTestWebhookEventLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建只投递到指定端点的测试事件。
func (l *CreateTestWebhookEventLogic) CreateTestWebhookEvent(in *core.CreateTestWebhookEventReq) (*core.RespBase, error) {
	return createTestWebhookEvent(l.ctx, l.svcCtx, in)
}
