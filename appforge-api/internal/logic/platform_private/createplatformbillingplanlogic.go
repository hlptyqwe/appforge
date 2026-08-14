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

type CreatePlatformBillingPlanLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformBillingPlanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformBillingPlanLogic {
	return &CreatePlatformBillingPlanLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformBillingPlanLogic) CreatePlatformBillingPlan(req *types.CreatePlatformBillingPlanReq) (resp *types.PlatformBillingPlanResp, err error) {
	return logicutil.Proxy[types.PlatformBillingPlanResp](l.ctx, req, l.svcCtx.CoreCli.CreateBillingPlan)
}
