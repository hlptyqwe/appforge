package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TTenantSubscriptionModel = (*customTTenantSubscriptionModel)(nil)

type (
	// TTenantSubscriptionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTTenantSubscriptionModel.
	TTenantSubscriptionModel interface {
		tTenantSubscriptionModel
	}

	customTTenantSubscriptionModel struct {
		*defaultTTenantSubscriptionModel
	}
)

// NewTTenantSubscriptionModel returns a model for the database table.
func NewTTenantSubscriptionModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TTenantSubscriptionModel {
	return &customTTenantSubscriptionModel{
		defaultTTenantSubscriptionModel: newTTenantSubscriptionModel(conn, c, opts...),
	}
}
