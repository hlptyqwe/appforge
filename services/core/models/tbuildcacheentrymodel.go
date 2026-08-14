package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TBuildCacheEntryModel = (*customTBuildCacheEntryModel)(nil)

type (
	// TBuildCacheEntryModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTBuildCacheEntryModel.
	TBuildCacheEntryModel interface {
		tBuildCacheEntryModel
	}

	customTBuildCacheEntryModel struct {
		*defaultTBuildCacheEntryModel
	}
)

// NewTBuildCacheEntryModel returns a model for the database table.
func NewTBuildCacheEntryModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TBuildCacheEntryModel {
	return &customTBuildCacheEntryModel{
		defaultTBuildCacheEntryModel: newTBuildCacheEntryModel(conn, c, opts...),
	}
}
