// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformSourceIntegrationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformSourceIntegrationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformSourceIntegrationsLogic {
	return &ListPlatformSourceIntegrationsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformSourceIntegrationsLogic) ListPlatformSourceIntegrations(req *types.ListPlatformSourceIntegrationsReq) (resp *types.PlatformSourceIntegrationListResp, err error) {
	return listPlatformSourceIntegrations(l.ctx, l.svcCtx, req)
}
