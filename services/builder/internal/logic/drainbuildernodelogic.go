package logic

import (
	"context"

	"appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DrainBuilderNodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDrainBuilderNodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DrainBuilderNodeLogic {
	return &DrainBuilderNodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 修改V4 Builder节点排空状态。
func (l *DrainBuilderNodeLogic) DrainBuilderNode(in *builder.DrainBuilderNodeReq) (*builder.BuilderNodeResp, error) {
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.DrainBuilderNode(toCoreContext(l.ctx), &core.DrainBuilderNodeReq{
		Id: in.GetId(), NodeCode: in.GetNodeCode(), DrainStatus: core.BuilderDrainStatus(in.GetDrainStatus()),
	})
	if err != nil {
		return nil, err
	}
	return &builder.BuilderNodeResp{Base: resp.Base, Data: mapBuilderNode(resp.Data)}, nil
}
