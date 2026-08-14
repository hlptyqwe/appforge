package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RecoverBuilderNodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRecoverBuilderNodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RecoverBuilderNodeLogic {
	return &RecoverBuilderNodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 在心跳、失败次数和磁盘容量均恢复后人工解除V4 Builder节点隔离。
func (l *RecoverBuilderNodeLogic) RecoverBuilderNode(in *core.RecoverBuilderNodeReq) (*core.BuilderNodeResp, error) {
	return recoverBuilderNode(l.ctx, l.svcCtx, in)
}
