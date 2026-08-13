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

type SysTenantListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSysTenantListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantListLogic {
	return &SysTenantListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SysTenantListLogic) SysTenantList(req *types.SysTenantListReq) (resp *types.SysTenantListResp, err error) {
	return logicutil.Proxy[types.SysTenantListResp](l.ctx, req, l.svcCtx.SystemCli.SysTenantList)
}
