package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysRoleGrantLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysRoleGrantLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysRoleGrantLogic {
	return &SysRoleGrantLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysRoleGrantLogic) SysRoleGrant(in *system.SysRoleGrantReq) (*system.RespBase, error) {
	if in == nil || in.RoleId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "role_id is required")
	}
	role, err := l.svcCtx.RoleModel.FindOne(l.ctx, in.RoleId)
	if err != nil {
		return nil, notFound(err, "role")
	}
	if err := l.svcCtx.RoleMenuModel.DeleteByRoleId(l.ctx, in.RoleId); err != nil {
		return nil, status.Errorf(codes.Internal, "clear role permissions failed: %v", err)
	}
	items := make([]*models.SysRoleMenu, 0, len(in.MenuIds))
	for _, menuID := range in.MenuIds {
		if menuID <= 0 {
			continue
		}
		items = append(items, &models.SysRoleMenu{TenantId: role.TenantId, RoleId: in.RoleId, MenuId: menuID})
	}
	if err := l.svcCtx.RoleMenuModel.InsertBatch(l.ctx, items); err != nil {
		return nil, status.Errorf(codes.Internal, "save role permissions failed: %v", err)
	}

	return &system.RespBase{Base: responseBase()}, nil
}
