package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

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
func (l *DrainBuilderNodeLogic) DrainBuilderNode(in *core.DrainBuilderNodeReq) (*core.BuilderNodeResp, error) {
	return drainBuilderNode(l.ctx, l.svcCtx, in)
}
