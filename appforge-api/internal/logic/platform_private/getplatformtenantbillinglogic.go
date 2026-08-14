// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/logicutil"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformTenantBillingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformTenantBillingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformTenantBillingLogic {
	return &GetPlatformTenantBillingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformTenantBillingLogic) GetPlatformTenantBilling(req *types.PlatformTenantBillingReq) (resp *types.PlatformTenantBillingResp, err error) {
	return logicutil.Proxy[types.PlatformTenantBillingResp](l.ctx, req, l.svcCtx.CoreCli.GetTenantBilling)
}
