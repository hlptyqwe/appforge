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

type RetirePlatformBillingPlanLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRetirePlatformBillingPlanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetirePlatformBillingPlanLogic {
	return &RetirePlatformBillingPlanLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RetirePlatformBillingPlanLogic) RetirePlatformBillingPlan(req *types.PlatformBillingPlanIdReq) (resp *types.PlatformBillingPlanResp, err error) {
	return logicutil.Proxy[types.PlatformBillingPlanResp](l.ctx, req, l.svcCtx.CoreCli.RetireBillingPlan)
}
