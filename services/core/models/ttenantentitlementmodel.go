package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TTenantEntitlementModel = (*customTTenantEntitlementModel)(nil)

type (
	// TTenantEntitlementModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTTenantEntitlementModel.
	TTenantEntitlementModel interface {
		tTenantEntitlementModel
	}

	customTTenantEntitlementModel struct {
		*defaultTTenantEntitlementModel
	}
)

// NewTTenantEntitlementModel returns a model for the database table.
func NewTTenantEntitlementModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TTenantEntitlementModel {
	return &customTTenantEntitlementModel{
		defaultTTenantEntitlementModel: newTTenantEntitlementModel(conn, c, opts...),
	}
}
