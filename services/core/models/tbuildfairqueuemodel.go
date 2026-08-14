package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TBuildFairQueueModel = (*customTBuildFairQueueModel)(nil)

type (
	// TBuildFairQueueModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTBuildFairQueueModel.
	TBuildFairQueueModel interface {
		tBuildFairQueueModel
	}

	customTBuildFairQueueModel struct {
		*defaultTBuildFairQueueModel
	}
)

// NewTBuildFairQueueModel returns a model for the database table.
func NewTBuildFairQueueModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TBuildFairQueueModel {
	return &customTBuildFairQueueModel{
		defaultTBuildFairQueueModel: newTBuildFairQueueModel(conn, c, opts...),
	}
}
