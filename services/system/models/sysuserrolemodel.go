package models

import (
	"appforge/common/sqlutil"
	"context"
	"database/sql"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
)

var _ SysUserRoleModel = (*customSysUserRoleModel)(nil)

type (
	// SysUserRoleModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysUserRoleModel.
	SysUserRoleModel interface {
		sysUserRoleModel
		FindIdsByTenantId(ctx context.Context, tenantId int64) ([]int64, error)
		FindRoleIdsByUserId(ctx context.Context, userId int64) ([]int64, error)
		FindRoleIdsByUserIds(ctx context.Context, userIds []int64) (map[int64][]int64, error)
		InsertCtx(ctx context.Context, session sqlx.Session, data *SysUserRole) (sql.Result, error)
		FindLoginUserPerms(ctx context.Context, userId int64, clear bool) ([]string, error)
		FindByIds(ctx context.Context, userId int64, roleIds []int64) ([]int64, error)
		FindByRoleId(ctx context.Context, roleId int64) ([]int64, error)
	}

	customSysUserRoleModel struct {
		*defaultSysUserRoleModel
	}
)

// NewSysUserRoleModel returns a model for the database table.
func NewSysUserRoleModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysUserRoleModel {
	return &customSysUserRoleModel{
		defaultSysUserRoleModel: newSysUserRoleModel(conn, c, opts...),
	}
}

func (m *defaultSysUserRoleModel) FindIdsByTenantId(ctx context.Context, tenantId int64) ([]int64, error) {
	builder := sqlutil.NewPageQueryBuilder()
	builder.EqInt64("tenant_id", tenantId)

	var ids []int64
	query := fmt.Sprintf("select id from %s where %s", m.table, builder.Where())
	err := m.QueryRowsNoCacheCtx(ctx, &ids, query, builder.Args()...)
	return ids, err
}

func (m *defaultSysUserRoleModel) FindRoleIdsByUserId(ctx context.Context, userId int64) ([]int64, error) {
	builder := sqlutil.NewPageQueryBuilder()
	builder.And("user_id = ?", userId)

	var ids []int64
	query := fmt.Sprintf("select role_id from %s where %s", m.table, builder.Where())
	err := m.QueryRowsNoCacheCtx(ctx, &ids, query, builder.Args()...)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (m *defaultSysUserRoleModel) FindRoleIdsByUserIds(
	ctx context.Context,
	userIds []int64,
) (map[int64][]int64, error) {

	if len(userIds) == 0 {
		return map[int64][]int64{}, nil
	}

	placeholders := make([]string, 0, len(userIds))
	args := make([]any, 0, len(userIds))
	for _, id := range userIds {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	query := fmt.Sprintf(
		"SELECT user_id, role_id FROM %s WHERE user_id IN (%s)",
		m.table,
		strings.Join(placeholders, ","),
	)

	type row struct {
		UserId int64
		RoleId int64
	}

	var rows []row

	err := m.QueryRowsNoCacheCtx(ctx, &rows, query, args...)
	if err != nil {
		return nil, err
	}

	mapping := make(map[int64][]int64)
	for _, r := range rows {
		mapping[r.UserId] = append(mapping[r.UserId], r.RoleId)
	}

	return mapping, nil
}

func (m *defaultSysUserRoleModel) InsertCtx(ctx context.Context, session sqlx.Session, data *SysUserRole) (sql.Result, error) {
	query := fmt.Sprintf("insert into %s (`tenant_id`, `user_id`, `role_id`) values (?, ?, ?)", m.table)
	ret, err := session.ExecCtx(ctx, query, data.TenantId, data.UserId, data.RoleId)
	return ret, err
}

func (m *defaultSysUserRoleModel) FindLoginUserPerms(ctx context.Context, userId int64, clear bool) ([]string, error) {
	var permsVer int64
	if err := m.QueryRowNoCacheCtx(ctx, &permsVer, "SELECT perms_ver FROM sys_user WHERE id = ?", userId); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("system:user:perms:%d:v%d", userId, permsVer)
	var perms []string
	if clear {
		return m.findLoginUserPermsFromDB(ctx, userId, key)
	} else {
		err := m.GetCacheCtx(ctx, key, &perms)
		if err != nil {
			return m.findLoginUserPermsFromDB(ctx, userId, key)
		}
	}
	return perms, nil
}

func (m *defaultSysUserRoleModel) findLoginUserPermsFromDB(ctx context.Context, userId int64, key string) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT m.perms
		FROM %s ur
		INNER JOIN sys_role_menu rm ON ur.role_id = rm.role_id
		INNER JOIN sys_user u ON ur.user_id = u.id
		INNER JOIN sys_role r ON ur.role_id = r.id
		INNER JOIN sys_menu m ON rm.menu_id = m.id
		WHERE ur.user_id = ? AND m.perms != ''
		  AND u.app_scope = r.app_scope
		  AND r.app_scope = m.app_scope
	`, m.table)
	var perms []string
	err := m.QueryRowsNoCacheCtx(ctx, &perms, query, userId)
	if err != nil {
		return nil, err
	}
	m.SetCacheCtx(ctx, key, perms)
	return perms, nil
}

func (m *defaultSysUserRoleModel) FindByIds(ctx context.Context, userId int64, roleIds []int64) ([]int64, error) {
	if len(roleIds) == 0 {
		return []int64{}, nil
	}

	builder := sqlutil.NewPageQueryBuilder()
	builder.And("user_id = ?", userId)
	builder.InInt64("role_id", roleIds)

	var ids []int64
	query := fmt.Sprintf("SELECT id FROM %s WHERE %s", m.table, builder.Where())
	err := m.QueryRowsNoCacheCtx(ctx, &ids, query, builder.Args()...)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (m *defaultSysUserRoleModel) FindByRoleId(ctx context.Context, roleId int64) ([]int64, error) {
	builder := sqlutil.NewPageQueryBuilder()
	builder.And("role_id = ?", roleId)

	var ids []int64
	query := fmt.Sprintf("SELECT user_id FROM %s WHERE %s", m.table, builder.Where())
	err := m.QueryRowsNoCacheCtx(ctx, &ids, query, builder.Args()...)
	if err != nil {
		return nil, err
	}
	return ids, nil
}
