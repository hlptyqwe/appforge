// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package system

import (
	"context"

	"appforge/admin-api/internal/logicutil"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/common"
	systempb "appforge/proto/system"

	"github.com/zeromicro/go-zero/core/logx"
)

type SysMenuUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSysMenuUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysMenuUpdateLogic {
	return &SysMenuUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SysMenuUpdateLogic) SysMenuUpdate(req *types.SysMenuUpdateReq) (resp *types.RespBase, err error) {
	return logicutil.Proxy[types.RespBase](l.ctx, &systempb.SysMenuUpdateReq{
		Id:        req.Id,
		ParentId:  req.ParentId,
		Name:      req.Name,
		MenuType:  toMenuType(req.MenuType),
		Method:    toRequestMethod(req.Method),
		Path:      req.Path,
		Component: req.Component,
		Icon:      req.Icon,
		Sort:      req.Sort,
		Visible:   common.Switch(req.Visible),
		Enabled:   common.Enable(req.Enabled),
		Perms:     req.Perms,
		AppScope:  systempb.ApplicationScope(req.AppScope),
	}, l.svcCtx.SystemCli.SysMenuUpdate)
}
