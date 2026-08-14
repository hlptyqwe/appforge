package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBuilderNodesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBuilderNodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBuilderNodesLogic {
	return &ListBuilderNodesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询V4 Builder集群节点。
func (l *ListBuilderNodesLogic) ListBuilderNodes(in *core.BuilderNodeListReq) (*core.BuilderNodeListResp, error) {
	return listBuilderNodes(l.ctx, l.svcCtx, in)
}
