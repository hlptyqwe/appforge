package adminlogic

import (
	"context"

	"appforge/proto/common"
	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProfileLogic {
	return &GetProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProfileLogic) GetProfile(in *system.Empty) (*system.ProfileResp, error) {
	userID := actorID(l.ctx)
	if userID <= 0 {
		return nil, status.Error(codes.Unauthenticated, "login user is required")
	}

	user, err := l.svcCtx.UserModel.FindOne(l.ctx, userID)
	if err != nil {
		return nil, notFound(err, "user")
	}
	if user.Enabled != int64(common.Enable_ENABLE_ENABLED) {
		return nil, status.Error(codes.PermissionDenied, "user is disabled")
	}

	var roleIDs []int64
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &roleIDs, `
		SELECT DISTINCT r.id
		FROM sys_user_role ur
		JOIN sys_role r ON r.id = ur.role_id
		WHERE ur.user_id = ? AND r.tenant_id = ? AND r.app_scope = ? AND r.enabled = 1
		ORDER BY r.id`, user.Id, user.TenantId, user.AppScope); err != nil {
		return nil, status.Errorf(codes.Internal, "query user roles failed: %v", err)
	}

	var perms []string
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &perms, `
		SELECT DISTINCT m.perms
		FROM sys_user_role ur
		JOIN sys_role r ON r.id = ur.role_id
		JOIN sys_role_menu rm ON rm.role_id = r.id
		JOIN sys_menu m ON m.id = rm.menu_id
		WHERE ur.user_id = ? AND r.tenant_id = ? AND r.app_scope = ?
		  AND r.enabled = 1 AND m.enabled = 1 AND m.perms <> ''
		ORDER BY m.perms`, user.Id, user.TenantId, user.AppScope); err != nil {
		return nil, status.Errorf(codes.Internal, "query user permissions failed: %v", err)
	}

	var menuRows []models.SysMenu
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &menuRows, `
		SELECT DISTINCT m.id, m.parent_id, m.app_scope, m.name, m.menu_type,
		       m.method, m.path, m.component, m.perms, m.icon, m.sort,
		       m.visible, m.enabled, m.create_times, m.update_times
		FROM sys_user_role ur
		JOIN sys_role r ON r.id = ur.role_id
		JOIN sys_role_menu rm ON rm.role_id = r.id
		JOIN sys_menu m ON m.id = rm.menu_id
		WHERE ur.user_id = ? AND r.tenant_id = ? AND r.app_scope = ?
		  AND r.enabled = 1 AND m.enabled = 1 AND m.menu_type IN (1, 2)
		ORDER BY m.sort, m.id`, user.Id, user.TenantId, user.AppScope); err != nil {
		return nil, status.Errorf(codes.Internal, "query user menus failed: %v", err)
	}

	return &system.ProfileResp{
		Base: responseBase(),
		Data: &system.ProfileData{
			User: &system.ProfileUser{
				Id:               user.Id,
				Username:         user.Username,
				Nickname:         user.Nickname,
				Avatar:           user.Avatar,
				TenantId:         user.TenantId,
				UserType:         system.UserType(user.UserType),
				IsOwner:          common.YesNo(user.IsOwner),
				Google2FaEnabled: common.Enable(user.GoogleEnabled),
				AppScope:         system.ApplicationScope(user.AppScope),
			},
			Menus:   buildProfileMenuTree(menuRows),
			Perms:   perms,
			RoleIds: roleIDs,
		},
	}, nil
}
