package logic

import (
	"context"

	"appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/svc"

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

// 结合节点能力、公平队列和并发槽位领取V4构建任务。
func (l *ClaimScheduledBuildTaskLogic) ClaimScheduledBuildTask(in *builder.ClaimScheduledBuildTaskReq) (*builder.BuildTaskResp, error) {
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.ClaimScheduledBuildTask(toCoreContext(l.ctx), &core.ClaimScheduledBuildTaskReq{
		NodeCode: in.GetNodeCode(), LeaseSeconds: in.GetLeaseSeconds(),
	})
	if err != nil {
		return nil, err
	}
	return &builder.BuildTaskResp{Base: resp.Base, Data: mapBuildTask(resp.Data)}, nil
}
