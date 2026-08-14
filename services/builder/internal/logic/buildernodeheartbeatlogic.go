package logic

import (
	"context"

	"appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type BuilderNodeHeartbeatLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBuilderNodeHeartbeatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BuilderNodeHeartbeatLogic {
	return &BuilderNodeHeartbeatLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 上报V4 Builder节点心跳和容量。
func (l *BuilderNodeHeartbeatLogic) BuilderNodeHeartbeat(in *builder.BuilderNodeHeartbeatReq) (*builder.BuilderNodeResp, error) {
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.BuilderNodeHeartbeat(toCoreContext(l.ctx), &core.BuilderNodeHeartbeatReq{
		NodeCode: in.GetNodeCode(), RunningCount: in.GetRunningCount(), DiskFree: in.GetDiskFree(),
		RunningTaskIds: in.GetRunningTaskIds(), LastErrorMessage: in.GetLastErrorMessage(),
	})
	if err != nil {
		return nil, err
	}
	return &builder.BuilderNodeResp{Base: resp.Base, Data: mapBuilderNode(resp.Data)}, nil
}
