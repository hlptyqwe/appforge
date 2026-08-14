package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CompleteLocalAgentBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCompleteLocalAgentBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteLocalAgentBuildTaskLogic {
	return &CompleteLocalAgentBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Agent校验Artifact后完成构建任务。
func (l *CompleteLocalAgentBuildTaskLogic) CompleteLocalAgentBuildTask(in *core.CompleteLocalAgentBuildTaskReq) (*core.RespBase, error) {
	return completeLocalAgentBuildTask(l.ctx, l.svcCtx, in)
}
