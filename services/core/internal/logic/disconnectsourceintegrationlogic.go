package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DisconnectSourceIntegrationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDisconnectSourceIntegrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisconnectSourceIntegrationLogic {
	return &DisconnectSourceIntegrationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 断开集成并使令牌立即不可用。
func (l *DisconnectSourceIntegrationLogic) DisconnectSourceIntegration(in *core.SourceIntegrationIdReq) (*core.SourceIntegrationResp, error) {
	return disconnectSourceIntegration(l.ctx, l.svcCtx, in)
}
