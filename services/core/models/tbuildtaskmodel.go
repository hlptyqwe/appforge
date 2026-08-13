package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TBuildTaskModel = (*customTBuildTaskModel)(nil)

type (
	// TBuildTaskModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTBuildTaskModel.
	TBuildTaskModel interface {
		tBuildTaskModel
	}

	customTBuildTaskModel struct {
		*defaultTBuildTaskModel
	}
)

// NewTBuildTaskModel returns a model for the database table.
func NewTBuildTaskModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TBuildTaskModel {
	return &customTBuildTaskModel{
		defaultTBuildTaskModel: newTBuildTaskModel(conn, c, opts...),
	}
}
