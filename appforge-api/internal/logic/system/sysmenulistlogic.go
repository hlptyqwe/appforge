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

type SysMenuListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSysMenuListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysMenuListLogic {
	return &SysMenuListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SysMenuListLogic) SysMenuList(req *types.SysMenuListReq) (resp *types.SysMenuListResp, err error) {
	result, err := l.svcCtx.SystemCli.SysMenuList(l.ctx, &systempb.SysMenuListReq{
		Page: &common.PageReq{
			Cursor: req.Cursor,
			Limit:  req.Limit,
			Count:  req.Count,
		},
		Keyword:  req.Keyword,
		MenuType: toMenuType(req.MenuType),
		Enabled:  toCommonStatus(req.Enabled),
		Visible:  toVisibleStatus(req.Visible),
		AppScope: systempb.ApplicationScope(req.AppScope),
	})
	if err != nil {
		return logicutil.SystemErrorResp[types.SysMenuListResp](l.ctx, err)
	}

	resp = &types.SysMenuListResp{Data: make([]types.SysMenuItem, 0, len(result.Data))}
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
