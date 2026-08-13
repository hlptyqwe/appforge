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

type SysMenuDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSysMenuDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysMenuDeleteLogic {
	return &SysMenuDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SysMenuDeleteLogic) SysMenuDelete(req *types.SysMenuDeleteReq) (resp *types.RespBase, err error) {
	return logicutil.Proxy[types.RespBase](l.ctx, req, l.svcCtx.SystemCli.SysMenuDelete)
}
