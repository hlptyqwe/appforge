package adminlogic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"appforge/common/utils"
	"appforge/proto/common"
	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func responseBase() *common.RespBase { return &common.RespBase{Code: 200, Msg: "OK"} }

func tenantID(ctx context.Context) int64 {
	if value, err := utils.GetTrustedTenantIdFromCtx(ctx); err == nil {
		return value
	}
	if value, err := utils.GetTenantIdFromMd(ctx); err == nil {
		return value
	}
	return 0
}

func actorID(ctx context.Context) int64 {
	if value, err := utils.GetUserIdFromCtx(ctx); err == nil {
		return value
	}
	if value, err := utils.GetUserIdFromMd(ctx); err == nil {
		return value
	}
	return 0
}

func positive(value int64, field string) error {
	if value <= 0 {
		return status.Errorf(codes.InvalidArgument, "%s must be greater than zero", field)
	}
	return nil
}

func menuItem(item *models.SysMenu) *system.SysMenuItem {
	if item == nil {
		return nil
	}
	return &system.SysMenuItem{
		Id: item.Id, ParentId: item.ParentId, Name: item.Name,
		MenuType: system.MenuType(item.MenuType), Method: requestMethod(item.Method),
		Path: item.Path, Component: item.Component, Icon: item.Icon, Sort: item.Sort,
		Visible: common.Switch(item.Visible), Enabled: common.Enable(item.Enabled),
		Perms: item.Perms, AppScope: system.ApplicationScope(item.AppScope),
	}
}

func userItem(item *models.SysUser, roleIDs []int64) *system.SysUserItem {
	if item == nil {
		return nil
	}
	return &system.SysUserItem{
		Id:               item.Id,
		Username:         item.Username,
		Nickname:         item.Nickname,
		Enabled:          common.Enable(item.Enabled),
		RoleIds:          roleIDs,
		CreateTimes:      item.CreateTimes,
		Google2FaEnabled: common.Enable(item.GoogleEnabled),
		TenantId:         item.TenantId,
		UserType:         system.UserType(item.UserType),
		IsOwner:          common.YesNo(item.IsOwner),
		Avatar:           item.Avatar,
		PermsVer:         item.PermsVer,
		LastLoginIp:      item.LastLoginIp,
		LastLoginAt:      item.LastLoginAt,
		CreateBy:         item.CreateBy,
		UpdateTimes:      item.UpdateTimes,
		AppScope:         system.ApplicationScope(item.AppScope),
	}
}

func configItem(item *models.SysConfig) *system.SysConfigItem {
	if item == nil {
		return nil
	}
	return &system.SysConfigItem{
		Id:          item.Id,
		TenantId:    item.TenantId,
		ConfigKey:   item.ConfigKey.String,
		ConfigValue: item.ConfigValue.String,
		Remark:      item.Remark.String,
		CreateTimes: item.CreateTimes,
		UpdateTimes: item.UpdateTimes,
	}
}

func profileMenuNode(item *models.SysMenu) *system.SysMenuNode {
	if item == nil {
		return nil
	}
	return &system.SysMenuNode{
		Id:        item.Id,
		ParentId:  item.ParentId,
		Name:      item.Name,
		MenuType:  system.MenuType(item.MenuType),
		Path:      item.Path,
		Component: item.Component,
		Icon:      item.Icon,
		Sort:      item.Sort,
		Visible:   common.Switch(item.Visible),
		Enabled:   common.Enable(item.Enabled),
		Perms:     item.Perms,
		AppScope:  system.ApplicationScope(item.AppScope),
	}
}

func buildProfileMenuTree(items []models.SysMenu) []*system.SysMenuNode {
	nodes := make(map[int64]*system.SysMenuNode, len(items))
	ordered := make([]*system.SysMenuNode, 0, len(items))
	for i := range items {
		if items[i].MenuType == int64(system.MenuType_MENU_TYPE_BUTTON) {
			continue
		}
		node := profileMenuNode(&items[i])
		node.Children = make([]*system.SysMenuNode, 0)
		nodes[node.Id] = node
		ordered = append(ordered, node)
	}

	roots := make([]*system.SysMenuNode, 0)
	for _, node := range ordered {
		parent, ok := nodes[node.ParentId]
		if node.ParentId <= 0 || !ok {
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	return roots
}

func requestMethod(value string) system.RequestMethod {
	switch strings.ToUpper(value) {
	case "GET":
		return system.RequestMethod_REQUEST_METHOD_GET
	case "POST":
		return system.RequestMethod_REQUEST_METHOD_POST
	case "PUT":
		return system.RequestMethod_REQUEST_METHOD_PUT
	case "DELETE":
		return system.RequestMethod_REQUEST_METHOD_DELETE
	default:
		return system.RequestMethod_REQUEST_METHOD_UNKNOWN
	}
}

func buildMenuTree(items []*system.SysMenuItem) []*system.SysMenuItem {
	return items
}

func roleMenuItems(ctx context.Context, svcCtx *svc.ServiceContext, roleID int64) ([]*system.SysMenuItem, error) {
	var rows []models.SysMenu
	query := "SELECT id, parent_id, app_scope, name, menu_type, method, path, component, perms, icon, sort, visible, enabled, create_times, update_times FROM sys_menu WHERE app_scope = ? AND enabled = ? ORDER BY sort ASC, id ASC"
	args := []any{int64(system.ApplicationScope_APPLICATION_SCOPE_ADMIN), int64(common.Enable_ENABLE_ENABLED)}
	if roleID > 0 {
		query = "SELECT m.id, m.parent_id, m.app_scope, m.name, m.menu_type, m.method, m.path, m.component, m.perms, m.icon, m.sort, m.visible, m.enabled, m.create_times, m.update_times FROM sys_menu m JOIN sys_role_menu rm ON rm.menu_id = m.id WHERE rm.role_id = ? AND m.app_scope = ? AND m.enabled = ? ORDER BY m.sort ASC, m.id ASC"
		args = []any{roleID, args[0], args[1]}
	}
	if err := svcCtx.DB.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, status.Errorf(codes.Internal, "query menu permissions failed: %v", err)
	}
	items := make([]*system.SysMenuItem, 0, len(rows))
	for i := range rows {
		items = append(items, menuItem(&rows[i]))
	}
	return items, nil
}

func tokenExpand(tenant, userType, appScope int64) string {
	value, _ := json.Marshal(map[string]int64{"tid": tenant, "userType": userType, "appScope": appScope})
	return string(value)
}

func notFound(err error, resource string) error {
	if err == models.ErrNotFound || err == sqlx.ErrNotFound {
		return status.Errorf(codes.NotFound, "%s not found", resource)
	}
	return status.Errorf(codes.Internal, "%s query failed: %v", resource, err)
}

func loginExpire(secret string, user *models.SysUser, cfg int64) (string, int64, error) {
	if strings.TrimSpace(secret) == "" {
		return "", 0, status.Error(codes.Internal, "jwt access secret is not configured")
	}
	if cfg <= 0 {
		cfg = 86400
	}
	exp := time.Now().Add(time.Duration(cfg) * time.Second)
	token, err := utils.GenToken(secret, user.Id, user.Username, tokenExpand(user.TenantId, user.UserType, user.AppScope), "appforge-system", time.Until(exp))
	if err != nil {
		return "", 0, fmt.Errorf("generate token failed: %w", err)
	}
	return token, exp.Unix(), nil
}
