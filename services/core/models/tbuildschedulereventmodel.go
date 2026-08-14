package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TBuildSchedulerEventModel = (*customTBuildSchedulerEventModel)(nil)

type (
	// TBuildSchedulerEventModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTBuildSchedulerEventModel.
	TBuildSchedulerEventModel interface {
		tBuildSchedulerEventModel
	}

	customTBuildSchedulerEventModel struct {
		*defaultTBuildSchedulerEventModel
	}
)

// NewTBuildSchedulerEventModel returns a model for the database table.
func NewTBuildSchedulerEventModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TBuildSchedulerEventModel {
	return &customTBuildSchedulerEventModel{
		defaultTBuildSchedulerEventModel: newTBuildSchedulerEventModel(conn, c, opts...),
	}
}
