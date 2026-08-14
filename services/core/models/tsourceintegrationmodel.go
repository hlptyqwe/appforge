package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TSourceIntegrationModel = (*customTSourceIntegrationModel)(nil)

type (
	// TSourceIntegrationModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTSourceIntegrationModel.
	TSourceIntegrationModel interface {
		tSourceIntegrationModel
	}

	customTSourceIntegrationModel struct {
		*defaultTSourceIntegrationModel
	}
)

// NewTSourceIntegrationModel returns a model for the database table.
func NewTSourceIntegrationModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TSourceIntegrationModel {
	return &customTSourceIntegrationModel{
		defaultTSourceIntegrationModel: newTSourceIntegrationModel(conn, c, opts...),
	}
}
