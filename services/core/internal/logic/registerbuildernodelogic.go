package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

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

// 注册或刷新V4 Builder节点能力。
func (l *RegisterBuilderNodeLogic) RegisterBuilderNode(in *core.RegisterBuilderNodeReq) (*core.BuilderNodeResp, error) {
	return registerBuilderNode(l.ctx, l.svcCtx, in)
}
