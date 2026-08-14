// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformSourceIntegrationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformSourceIntegrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformSourceIntegrationLogic {
	return &GetPlatformSourceIntegrationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformSourceIntegrationLogic) GetPlatformSourceIntegration(req *types.PlatformIdReq) (resp *types.PlatformSourceIntegrationResp, err error) {
	return getPlatformSourceIntegration(l.ctx, l.svcCtx, req)
}
