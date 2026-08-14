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

type ListPlatformBillingPlansLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformBillingPlansLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformBillingPlansLogic {
	return &ListPlatformBillingPlansLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformBillingPlansLogic) ListPlatformBillingPlans(req *types.ListPlatformBillingPlansReq) (resp *types.PlatformBillingPlanListResp, err error) {
	return logicutil.Proxy[types.PlatformBillingPlanListResp](l.ctx, req, l.svcCtx.CoreCli.ListBillingPlans)
}
