package models

import (
	"appforge/common/sqlutil"
	"context"
	"database/sql"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	g "github.com/zeromicro/go-zero/core/stores/sqlx"
	zero "github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
)

var _ SysRoleMenuModel = (*customSysRoleMenuModel)(nil)

type (
	// SysRoleMenuModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysRoleMenuModel.
	SysRoleMenuModel interface {
		sysRoleMenuModel
		FindMenuIdsByRoleIds(ctx context.Context, roleIds []int64) ([]int64, error)
		FindIdsByTenantId(ctx context.Context, tenantId int64) ([]int64, error)
		DeleteByRoleId(ctx context.Context, roleId int64) error
		InsertBatch(ctx context.Context, data []*SysRoleMenu) error
		TransactCtx(ctx context.Context, fn func(context.Context, g.Session) error) error
		ListByRoleId(ctx context.Context, roleId int64) ([]*SysRoleMenu, error)
	}

	customSysRoleMenuModel struct {
		*defaultSysRoleMenuModel
	}
)

// NewSysRoleMenuModel returns a model for the database table.
func NewSysRoleMenuModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysRoleMenuModel {
	return &customSysRoleMenuModel{
		defaultSysRoleMenuModel: newSysRoleMenuModel(conn, c, opts...),
	}
}

func (m *defaultSysRoleMenuModel) FindIdsByTenantId(ctx context.Context, tenantId int64) ([]int64, error) {
	builder := sqlutil.NewPageQueryBuilder()
	builder.EqInt64("tenant_id", tenantId)

	var ids []int64
	query := fmt.Sprintf("select id from %s where %s", m.table, builder.Where())
	err := m.QueryRowsNoCacheCtx(ctx, &ids, query, builder.Args()...)
	return ids, err
}

func (m *defaultSysRoleMenuModel) FindMenuIdsByRoleIds(ctx context.Context, roleIds []int64) ([]int64, error) {
	if len(roleIds) == 0 {
		return []int64{}, nil
	}

	builder := sqlutil.NewPageQueryBuilder()
	builder.InInt64("role_id", roleIds)

	var ids []int64
	query := fmt.Sprintf("select menu_id from %s where %s", m.table, builder.Where())
	err := m.QueryRowsNoCacheCtx(ctx, &ids, query, builder.Args()...)
	return ids, err
}

func (m *defaultSysRoleMenuModel) DeleteByRoleId(ctx context.Context, roleId int64) error {
	builder := sqlutil.NewPageQueryBuilder()
	builder.And("role_id = ?", roleId)

	query := fmt.Sprintf("DELETE FROM %s WHERE %s", m.table, builder.Where())
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn zero.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query, builder.Args()...)
	})
	return err
}

func (m *defaultSysRoleMenuModel) InsertBatch(ctx context.Context, data []*SysRoleMenu) error {
	if len(data) == 0 {
		return nil
	}

	valueStrings := make([]string, 0, len(data))
	valueArgs := make([]interface{}, 0, len(data)*3)

	for _, d := range data {
		valueStrings = append(valueStrings, "(?, ?, ?)")
		valueArgs = append(valueArgs, d.TenantId, d.RoleId, d.MenuId)
	}

	stmt := fmt.Sprintf(
		"INSERT INTO %s (tenant_id, role_id, menu_id) VALUES %s",
		m.table,
		strings.Join(valueStrings, ","),
	)

	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn zero.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, stmt, valueArgs...)
	})
	return err
}

func (m *defaultSysRoleMenuModel) TransactCtx(ctx context.Context, fn func(context.Context, g.Session) error) error {
	return m.CachedConn.TransactCtx(ctx, fn)
}

func (m *defaultSysRoleMenuModel) ListByRoleId(ctx context.Context, roleId int64) ([]*SysRoleMenu, error) {
	builder := sqlutil.NewPageQueryBuilder()
	builder.And("role_id = ?", roleId)

	var list []*SysRoleMenu
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s", sysRoleMenuRows, m.table, builder.Where())
	err := m.QueryRowsNoCacheCtx(ctx, &list, query, builder.Args()...)
	return list, err
}
