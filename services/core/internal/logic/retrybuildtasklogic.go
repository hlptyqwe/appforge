package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetryBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRetryBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetryBuildTaskLogic {
	return &RetryBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 根据不可变历史快照创建V4重试任务。
func (l *RetryBuildTaskLogic) RetryBuildTask(in *core.RetryBuildTaskReq) (*core.BuildTaskResp, error) {
	return retryBuildTask(l.ctx, l.svcCtx, in)
}
