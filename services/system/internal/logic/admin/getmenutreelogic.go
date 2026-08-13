package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMenuTreeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMenuTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMenuTreeLogic {
	return &GetMenuTreeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMenuTreeLogic) GetMenuTree(in *system.SysMenuTreeReq) (*system.SysMenuTreeResp, error) {
	roleID := int64(0)
	if in != nil {
		roleID = in.RoleId
	}
	items, err := roleMenuItems(l.ctx, l.svcCtx, roleID)
	if err != nil {
		return nil, err
	}

	return &system.SysMenuTreeResp{Base: responseBase(), Data: buildMenuTree(items)}, nil
}
