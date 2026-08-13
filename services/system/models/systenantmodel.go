package models

import (
	"appforge/common/sqlutil"
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SysTenantModel = (*customSysTenantModel)(nil)

type (
	TenantPageFilter struct {
		Keyword      string
		Status       int64
		TenantName   string
		TenantCode   string
		ContactName  string
		ContactPhone string
		IDs          []int64
	}

	// SysTenantModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysTenantModel.
	SysTenantModel interface {
		sysTenantModel
		FindPage(ctx context.Context, filter TenantPageFilter, cursor int64, limit int64) ([]*SysTenant, int64, error)
		FindByTenantCode(ctx context.Context, tenantCode string) (*SysTenant, error)
	}

	customSysTenantModel struct {
		*defaultSysTenantModel
	}
)

// NewSysTenantModel returns a model for the database table.
func NewSysTenantModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysTenantModel {
	return &customSysTenantModel{
		defaultSysTenantModel: newSysTenantModel(conn, c, opts...),
	}
}

func (m *customSysTenantModel) FindPage(ctx context.Context, filter TenantPageFilter, cursor int64, limit int64) ([]*SysTenant, int64, error) {
	limit = sqlutil.NormalizeLimit(limit)

	builder := sqlutil.NewPageQueryBuilder()
	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		builder.And("(tenant_name LIKE ? OR tenant_code LIKE ? OR contact_name LIKE ? OR contact_phone LIKE ?)", like, like, like, like)
	}
	builder.EqInt64("status", filter.Status)
	builder.EqString("tenant_name", filter.TenantName)
	builder.EqString("tenant_code", filter.TenantCode)
	builder.EqString("contact_name", filter.ContactName)
	builder.EqString("contact_phone", filter.ContactPhone)
	builder.InInt64("id", filter.IDs)

	where := builder.Where()
	args := builder.Args()

	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY id DESC LIMIT ?,?", sysTenantRows, m.table, where)
	listArgs := append(append([]any{}, args...), cursor, limit)

	var list []*SysTenant
	err := m.QueryRowsNoCacheCtx(ctx, &list, query, listArgs...)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM %s WHERE %s", m.table, where)
	err = m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func (m *customSysTenantModel) FindByTenantCode(ctx context.Context, tenantCode string) (*SysTenant, error) {
	builder := sqlutil.NewPageQueryBuilder()
	builder.And("tenant_code = ?", tenantCode)

	query := fmt.Sprintf("select %s from %s where %s limit 1", sysTenantRows, m.table, builder.Where())
	var resp SysTenant
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, builder.Args()...)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
