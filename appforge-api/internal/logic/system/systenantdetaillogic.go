// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package system

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"appforge/admin-api/internal/logicutil"

	"github.com/zeromicro/go-zero/core/logx"
)

type SysTenantDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSysTenantDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantDetailLogic {
	return &SysTenantDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SysTenantDetailLogic) SysTenantDetail(req *types.SysTenantDetailReq) (resp *types.SysTenantDetailResp, err error) {
	return logicutil.Proxy[types.SysTenantDetailResp](l.ctx, req, l.svcCtx.SystemCli.SysTenantDetail)
}
