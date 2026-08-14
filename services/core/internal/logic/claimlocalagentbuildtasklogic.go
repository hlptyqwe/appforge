package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClaimLocalAgentBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClaimLocalAgentBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimLocalAgentBuildTaskLogic {
	return &ClaimLocalAgentBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Agent领取经过租户、应用、能力和协议范围校验的构建任务。
func (l *ClaimLocalAgentBuildTaskLogic) ClaimLocalAgentBuildTask(in *core.ClaimLocalAgentBuildTaskReq) (*core.LocalAgentBuildTaskResp, error) {
	return claimLocalAgentBuildTask(l.ctx, l.svcCtx, in)
}
