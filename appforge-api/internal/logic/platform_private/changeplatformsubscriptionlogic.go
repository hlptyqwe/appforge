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

type ChangePlatformSubscriptionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChangePlatformSubscriptionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangePlatformSubscriptionLogic {
	return &ChangePlatformSubscriptionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChangePlatformSubscriptionLogic) ChangePlatformSubscription(req *types.ChangePlatformSubscriptionReq) (resp *types.PlatformTenantBillingResp, err error) {
	return logicutil.Proxy[types.PlatformTenantBillingResp](l.ctx, req, l.svcCtx.CoreCli.ChangeTenantSubscription)
}
