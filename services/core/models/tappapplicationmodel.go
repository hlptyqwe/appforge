package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TAppApplicationModel = (*customTAppApplicationModel)(nil)

type (
	// TAppApplicationModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTAppApplicationModel.
	TAppApplicationModel interface {
		tAppApplicationModel
	}

	customTAppApplicationModel struct {
		*defaultTAppApplicationModel
	}
)

// NewTAppApplicationModel returns a model for the database table.
func NewTAppApplicationModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TAppApplicationModel {
	return &customTAppApplicationModel{
		defaultTAppApplicationModel: newTAppApplicationModel(conn, c, opts...),
	}
}
