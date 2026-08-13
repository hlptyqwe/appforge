// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package system

import (
	"context"

	"appforge/admin-api/internal/logicutil"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	systempb "appforge/proto/system"

	"github.com/zeromicro/go-zero/core/logx"
)

type SysMenuTreeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSysMenuTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysMenuTreeLogic {
	return &SysMenuTreeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SysMenuTreeLogic) SysMenuTree(req *types.SysMenuTreeReq) (resp *types.SysMenuTreeResp, err error) {
	result, err := l.svcCtx.SystemCli.GetMenuTree(l.ctx, &systempb.SysMenuTreeReq{RoleId: req.RoleId})
	if err != nil {
		return logicutil.SystemErrorResp[types.SysMenuTreeResp](l.ctx, err)
	}

	resp = &types.SysMenuTreeResp{Data: make([]types.SysMenuItem, 0, len(result.Data))}
	if result.Base != nil {
		resp.Code = result.Base.Code
		resp.Msg = result.Base.Msg
		resp.Total = result.Base.Total
		resp.HasNext = result.Base.HasNext
		resp.HasPrev = result.Base.HasPrev
		resp.NextCursor = result.Base.NextCursor
		resp.PrevCursor = result.Base.PrevCursor
	}
	for _, item := range result.Data {
		if item != nil {
			resp.Data = append(resp.Data, mapSysMenuItem(item))
		}
	}

	return resp, nil
}
