package models

import (
	"appforge/common/sqlutil"
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SysRoleModel = (*customSysRoleModel)(nil)

type (
	RolePageFilter struct {
		Keyword  string
		TenantId int64
		Enabled  int64
		AppScope int64
	}

	// SysRoleModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysRoleModel.
	SysRoleModel interface {
		sysRoleModel
		FindPage(ctx context.Context, filter RolePageFilter, cursor int64, limit int64) ([]*SysRole, int64, error)
		FindIdsByIds(ctx context.Context, ids []int64) ([]int64, error)
		FindIdsByTenantId(ctx context.Context, tenantId int64) ([]int64, error)
	}

	customSysRoleModel struct {
		*defaultSysRoleModel
	}
)

// NewSysRoleModel returns a model for the database table.
func NewSysRoleModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysRoleModel {
	return &customSysRoleModel{
		defaultSysRoleModel: newSysRoleModel(conn, c, opts...),
	}
}

func (m *defaultSysRoleModel) FindPage(
	ctx context.Context,
	filter RolePageFilter,
	cursor int64,
	limit int64,
) ([]*SysRole, int64, error) {

	limit = sqlutil.NormalizeLimit(limit)

	// ---- WHERE 条件 ----
	builder := sqlutil.NewPageQueryBuilder()
	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		builder.And("(name LIKE ? OR code LIKE ?)", like, like)
	}
	builder.EqInt64("enabled", filter.Enabled)
	builder.EqInt64("tenant_id", filter.TenantId)
	builder.EqInt64("app_scope", filter.AppScope)

	where := builder.Where()
	args := builder.Args()

	// ---- total ----
	var total int64
	countSql := fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countSql, args...); err != nil {
		return nil, 0, err
	}

	// ---- list ----
	var listSql string
	listArgs := append([]any{}, args...)

	if cursor <= 0 {
		// 第一页
		listSql = fmt.Sprintf(
			`SELECT %s
			FROM %s
			WHERE %s
			ORDER BY id DESC
			LIMIT ?`,
			sysRoleRows, m.table, where,
		)
		listArgs = append(listArgs, limit)
	} else {
		// 后续页
		listSql = fmt.Sprintf(
			`SELECT %s
			FROM %s
			WHERE %s AND id < ?
			ORDER BY id DESC
			LIMIT ?`,
			sysRoleRows, m.table, where,
		)
		listArgs = append(listArgs, cursor, limit)
	}

	var list []*SysRole
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listSql, listArgs...); err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func (m *defaultSysRoleModel) FindIdsByTenantId(ctx context.Context, tenantId int64) ([]int64, error) {
	builder := sqlutil.NewPageQueryBuilder()
	builder.EqInt64("tenant_id", tenantId)

	var ids []int64
	query := fmt.Sprintf("SELECT id FROM %s WHERE %s", m.table, builder.Where())
	err := m.QueryRowsNoCacheCtx(ctx, &ids, query, builder.Args()...)
	return ids, err
}

func (m *defaultSysRoleModel) FindIdsByIds(ctx context.Context, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return []int64{}, nil
	}

	builder := sqlutil.NewPageQueryBuilder()
	builder.InInt64("id", ids)

	var existIds []int64
	query := fmt.Sprintf("SELECT id FROM %s WHERE %s", m.table, builder.Where())
	err := m.QueryRowsNoCacheCtx(ctx, &existIds, query, builder.Args()...)
	if err != nil {
		return nil, err
	}
	return existIds, nil
}
