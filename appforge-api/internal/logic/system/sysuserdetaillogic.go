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

type SysUserDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSysUserDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysUserDetailLogic {
	return &SysUserDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SysUserDetailLogic) SysUserDetail(req *types.SysUserDetailReq) (resp *types.SysUserDetailResp, err error) {
	return logicutil.Proxy[types.SysUserDetailResp](l.ctx, req, l.svcCtx.SystemCli.SysUserDetail)
}
