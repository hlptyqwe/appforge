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

type GetPlatformBillingUsageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformBillingUsageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformBillingUsageLogic {
	return &GetPlatformBillingUsageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformBillingUsageLogic) GetPlatformBillingUsage(req *types.PlatformBillingUsageReq) (resp *types.PlatformBillingUsageResp, err error) {
	return logicutil.Proxy[types.PlatformBillingUsageResp](l.ctx, req, l.svcCtx.CoreCli.GetBillingUsage)
}
