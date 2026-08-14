// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformBuilderNodesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformBuilderNodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformBuilderNodesLogic {
	return &ListPlatformBuilderNodesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformBuilderNodesLogic) ListPlatformBuilderNodes(req *types.ListPlatformBuilderNodesReq) (resp *types.PlatformBuilderNodeListResp, err error) {
	item, err := l.svcCtx.CoreCli.ListBuilderNodes(l.ctx, &core.BuilderNodeListReq{Page: platformlogic.PlatformPage(req.PageReq),
		PoolCode: req.PoolCode, Status: core.BuilderNodeStatus(req.Status),
		DrainStatus: core.BuilderDrainStatus(req.DrainStatus), Keyword: req.Keyword})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformBuilderNode, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, platformlogic.MapPlatformBuilderNode(value))
	}
	return &types.PlatformBuilderNodeListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}
