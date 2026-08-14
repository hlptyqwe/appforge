package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClaimScheduledBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClaimScheduledBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimScheduledBuildTaskLogic {
	return &ClaimScheduledBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 结合节点能力、公平队列和并发槽位原子领取V4构建任务。
func (l *ClaimScheduledBuildTaskLogic) ClaimScheduledBuildTask(in *core.ClaimScheduledBuildTaskReq) (*core.BuildTaskResp, error) {
	return claimScheduledTask(l.ctx, l.svcCtx, in)
}
