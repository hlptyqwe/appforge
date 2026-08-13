// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package system

import (
	"context"

	"appforge/admin-api/internal/logicutil"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SysTenantDomainListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSysTenantDomainListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantDomainListLogic {
	return &SysTenantDomainListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SysTenantDomainListLogic) SysTenantDomainList(req *types.SysTenantDomainListReq) (resp *types.SysTenantDomainListResp, err error) {
	return logicutil.Proxy[types.SysTenantDomainListResp](l.ctx, req, l.svcCtx.SystemCli.SysTenantDomainList)
}
