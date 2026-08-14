// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DisconnectPlatformSourceIntegrationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDisconnectPlatformSourceIntegrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisconnectPlatformSourceIntegrationLogic {
	return &DisconnectPlatformSourceIntegrationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DisconnectPlatformSourceIntegrationLogic) DisconnectPlatformSourceIntegration(req *types.PlatformIdReq) (resp *types.PlatformSourceIntegrationResp, err error) {
	return disconnectPlatformSourceIntegration(l.ctx, l.svcCtx, req)
}
