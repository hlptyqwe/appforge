package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBuilderNodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBuilderNodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBuilderNodeLogic {
	return &GetBuilderNodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询V4 Builder节点详情。
func (l *GetBuilderNodeLogic) GetBuilderNode(in *core.BuilderNodeIdReq) (*core.BuilderNodeResp, error) {
	return getBuilderNode(l.ctx, l.svcCtx, in)
}
