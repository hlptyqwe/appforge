package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TBuildConcurrencyPolicyModel = (*customTBuildConcurrencyPolicyModel)(nil)

type (
	// TBuildConcurrencyPolicyModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTBuildConcurrencyPolicyModel.
	TBuildConcurrencyPolicyModel interface {
		tBuildConcurrencyPolicyModel
	}

	customTBuildConcurrencyPolicyModel struct {
		*defaultTBuildConcurrencyPolicyModel
	}
)

// NewTBuildConcurrencyPolicyModel returns a model for the database table.
func NewTBuildConcurrencyPolicyModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TBuildConcurrencyPolicyModel {
	return &customTBuildConcurrencyPolicyModel{
		defaultTBuildConcurrencyPolicyModel: newTBuildConcurrencyPolicyModel(conn, c, opts...),
	}
}
