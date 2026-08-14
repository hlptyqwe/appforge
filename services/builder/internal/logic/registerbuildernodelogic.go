package logic

import (
	"context"

	"appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterBuilderNodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterBuilderNodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterBuilderNodeLogic {
	return &RegisterBuilderNodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 注册或刷新V4 Builder节点及能力。
func (l *RegisterBuilderNodeLogic) RegisterBuilderNode(in *builder.RegisterBuilderNodeReq) (*builder.BuilderNodeResp, error) {
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.RegisterBuilderNode(toCoreContext(l.ctx), &core.RegisterBuilderNodeReq{
		NodeCode: in.GetNodeCode(), PoolCode: in.GetPoolCode(), Endpoint: in.GetEndpoint(),
		MaxConcurrency: in.GetMaxConcurrency(), CpuCapacity: in.GetCpuCapacity(),
		MemoryCapacity: in.GetMemoryCapacity(), DiskCapacity: in.GetDiskCapacity(), DiskFree: in.GetDiskFree(),
		ToolchainVersion: in.GetToolchainVersion(), BuildProtocolVersion: in.GetBuildProtocolVersion(),
		CapabilityJson: in.GetCapabilityJson(),
	})
	if err != nil {
		return nil, err
	}
	return &builder.BuilderNodeResp{Base: resp.Base, Data: mapBuilderNode(resp.Data)}, nil
}
