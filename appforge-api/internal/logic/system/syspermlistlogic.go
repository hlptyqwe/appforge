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

type SysPermListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSysPermListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysPermListLogic {
	return &SysPermListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SysPermListLogic) SysPermList() (resp *types.SysPermListResp, err error) {
	return logicutil.Proxy[types.SysPermListResp](l.ctx, nil, l.svcCtx.SystemCli.SysPermList)
}
