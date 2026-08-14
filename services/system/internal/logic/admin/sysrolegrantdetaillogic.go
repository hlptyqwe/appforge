package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysRoleGrantDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysRoleGrantDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysRoleGrantDetailLogic {
	return &SysRoleGrantDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysRoleGrantDetailLogic) SysRoleGrantDetail(in *system.SysRoleGrantDetailReq) (*system.SysRoleGrantDetailResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "role_id is required")
	}
	role, err := l.svcCtx.RoleModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, notFound(err, "role")
	}
	if err := requireItemAppScope(l.ctx, role.AppScope); err != nil {
		return nil, err
	}
	items, err := l.svcCtx.RoleMenuModel.ListByRoleId(l.ctx, in.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query role permissions failed: %v", err)
	}
	menuIDs := make([]int64, 0, len(items))
	for _, item := range items {
		menuIDs = append(menuIDs, item.MenuId)
	}
	var permKeys []string
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &permKeys, "SELECT DISTINCT m.perms FROM sys_role_menu rm JOIN sys_menu m ON m.id = rm.menu_id WHERE rm.role_id = ? AND m.perms <> '' ORDER BY m.perms", in.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "query role permission keys failed: %v", err)
	}

	return &system.SysRoleGrantDetailResp{Base: responseBase(), Data: &system.SysRoleGrantDetailData{RoleId: in.Id, MenuIds: menuIDs, PermKeys: permKeys}}, nil
}
