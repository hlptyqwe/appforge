package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TBuilderNodeModel = (*customTBuilderNodeModel)(nil)

type (
	// TBuilderNodeModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTBuilderNodeModel.
	TBuilderNodeModel interface {
		tBuilderNodeModel
	}

	customTBuilderNodeModel struct {
		*defaultTBuilderNodeModel
	}
)

// NewTBuilderNodeModel returns a model for the database table.
func NewTBuilderNodeModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TBuilderNodeModel {
	return &customTBuilderNodeModel{
		defaultTBuilderNodeModel: newTBuilderNodeModel(conn, c, opts...),
	}
}
