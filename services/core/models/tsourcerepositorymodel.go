package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TSourceRepositoryModel = (*customTSourceRepositoryModel)(nil)

type (
	// TSourceRepositoryModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTSourceRepositoryModel.
	TSourceRepositoryModel interface {
		tSourceRepositoryModel
	}

	customTSourceRepositoryModel struct {
		*defaultTSourceRepositoryModel
	}
)

// NewTSourceRepositoryModel returns a model for the database table.
func NewTSourceRepositoryModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TSourceRepositoryModel {
	return &customTSourceRepositoryModel{
		defaultTSourceRepositoryModel: newTSourceRepositoryModel(conn, c, opts...),
	}
}
