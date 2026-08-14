package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type FailLocalAgentBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFailLocalAgentBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FailLocalAgentBuildTaskLogic {
	return &FailLocalAgentBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Agent记录脱敏失败摘要并结束任务。
func (l *FailLocalAgentBuildTaskLogic) FailLocalAgentBuildTask(in *core.FailLocalAgentBuildTaskReq) (*core.RespBase, error) {
	return failLocalAgentBuildTask(l.ctx, l.svcCtx, in)
}
