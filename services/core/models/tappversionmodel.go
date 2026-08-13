package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TAppVersionModel = (*customTAppVersionModel)(nil)

type (
	// TAppVersionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTAppVersionModel.
	TAppVersionModel interface {
		tAppVersionModel
	}

	customTAppVersionModel struct {
		*defaultTAppVersionModel
	}
)

// NewTAppVersionModel returns a model for the database table.
func NewTAppVersionModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TAppVersionModel {
	return &customTAppVersionModel{
		defaultTAppVersionModel: newTAppVersionModel(conn, c, opts...),
	}
}
