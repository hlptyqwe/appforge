package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CompleteSourceWebhookEventLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCompleteSourceWebhookEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteSourceWebhookEventLogic {
	return &CompleteSourceWebhookEventLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Worker记录Artifact导入和构建任务创建成功。
func (l *CompleteSourceWebhookEventLogic) CompleteSourceWebhookEvent(in *core.CompleteSourceWebhookEventReq) (*core.RespBase, error) {
	return completeSourceWebhookEvent(l.ctx, l.svcCtx, in)
}
