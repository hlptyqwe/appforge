package adminlogic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
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

func responsePage(total int64, hasNext bool, nextCursor int64, hasPrev bool, prevCursor int64) *common.RespBase {
	return &common.RespBase{
		Code: 200, Msg: "OK", Total: total, HasNext: hasNext, HasPrev: hasPrev,
		NextCursor: nextCursor, PrevCursor: prevCursor,
	}
}

func pageValues(page *common.PageReq) (int64, int64) {
	if page == nil {
		return 0, 20
	}
	cursor := page.Cursor
	limit := page.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return cursor, limit
}

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

func roleItem(item *models.SysRole) *system.SysRoleItem {
	if item == nil {
		return nil
	}
	return &system.SysRoleItem{
		Id: item.Id, Name: item.Name, Code: item.Code,
		Enabled: common.Enable(item.Enabled), Remark: item.Remark,
		CreateTimes: item.CreateTimes, TenantId: item.TenantId, UpdateTimes: item.UpdateTimes,
		AppScope: system.ApplicationScope(item.AppScope),
	}
}

func tenantItem(item *models.SysTenant) *system.SysTenantItem {
	if item == nil {
		return nil
	}
	return &system.SysTenantItem{
		Id: item.Id, TenantCode: item.TenantCode, TenantName: item.TenantName,
		Enabled: common.Enable(item.Enabled), ExpireTime: item.ExpireTime,
		ContactName: item.ContactName.String, ContactPhone: item.ContactPhone.String,
		Remark: item.Remark.String, CreateTimes: item.CreateTimes, UpdateTimes: item.UpdateTimes,
		LoginIp: item.LoginIp.String, LoginTime: item.LoginTime, LoginCount: item.LoginCount,
	}
}

func domainItem(item *models.SysTenantDomain) *system.SysTenantDomainItem {
	if item == nil {
		return nil
	}
	return &system.SysTenantDomainItem{
		Id: item.Id, TenantId: item.TenantId, Origin: item.Origin,
		Status: system.TenantDomainStatus(item.Status), Priority: item.Priority,
		CreateTimes: item.CreateTimes, UpdateTimes: item.UpdateTimes,
	}
}

func nullText(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func effectiveAppScope(ctx context.Context, value system.ApplicationScope) int64 {
	if scope, err := utils.GetAppScopeFromMd(ctx); err == nil && scope > 0 {
		return scope
	}
	if value == system.ApplicationScope_APPLICATION_SCOPE_UNKNOWN {
		return int64(system.ApplicationScope_APPLICATION_SCOPE_ADMIN)
	}
	return int64(value)
}

func requireItemAppScope(ctx context.Context, itemScope int64) error {
	if scope, err := utils.GetAppScopeFromMd(ctx); err == nil && scope > 0 && scope != itemScope {
		return status.Error(codes.PermissionDenied, "cross-application-scope access is not allowed")
	}
	return nil
}

func effectiveTenant(ctx context.Context, requested int64) (int64, error) {
	if requested < 0 {
		return 0, status.Error(codes.InvalidArgument, "tenant_id is invalid")
	}
	current := tenantID(ctx)
	if current > 0 {
		if requested > 0 && requested != current {
			return 0, status.Error(codes.PermissionDenied, "cross-tenant access is not allowed")
		}
		return current, nil
	}
	return requested, nil
}

func normalizeOrigin(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", status.Error(codes.InvalidArgument, "origin is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", status.Error(codes.InvalidArgument, "origin must be a valid http or https URL")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.Scheme+"://"+parsed.Host+parsed.Path, "/"), nil
}

func replaceUserRoles(ctx context.Context, svcCtx *svc.ServiceContext, user *models.SysUser, roleIDs []int64) error {
	if _, err := svcCtx.DB.ExecCtx(ctx, "DELETE FROM sys_user_role WHERE user_id = ?", user.Id); err != nil {
		return status.Errorf(codes.Internal, "clear user roles failed: %v", err)
	}
	seen := make(map[int64]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		if roleID <= 0 {
			continue
		}
		if _, ok := seen[roleID]; ok {
			continue
		}
		seen[roleID] = struct{}{}
		var role models.SysRole
		if err := svcCtx.DB.QueryRowCtx(ctx, &role, "SELECT id, tenant_id, app_scope, name, code, enabled, remark, create_times, update_times FROM sys_role WHERE id = ? LIMIT 1", roleID); err != nil {
			return status.Errorf(codes.InvalidArgument, "role %d not found", roleID)
		}
		if role.TenantId != user.TenantId || role.AppScope != user.AppScope {
			return status.Errorf(codes.InvalidArgument, "role %d is outside user scope", roleID)
		}
		if _, err := svcCtx.UserRoleModel.Insert(ctx, &models.SysUserRole{TenantId: user.TenantId, UserId: user.Id, RoleId: roleID}); err != nil {
			return status.Errorf(codes.Internal, "assign user role failed: %v", err)
		}
	}
	return nil
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
	appScope := effectiveAppScope(ctx, system.ApplicationScope_APPLICATION_SCOPE_UNKNOWN)
	if roleID > 0 {
		role, err := svcCtx.RoleModel.FindOne(ctx, roleID)
		if err != nil {
			return nil, notFound(err, "role")
		}
		if err := requireItemAppScope(ctx, role.AppScope); err != nil {
			return nil, err
		}
		appScope = role.AppScope
	}
	query := "SELECT id, parent_id, app_scope, name, menu_type, method, path, component, perms, icon, sort, visible, enabled, create_times, update_times FROM sys_menu WHERE app_scope = ? AND enabled = ? ORDER BY sort ASC, id ASC"
	args := []any{appScope, int64(common.Enable_ENABLE_ENABLED)}
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
