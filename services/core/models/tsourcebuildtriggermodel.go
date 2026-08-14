package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TSourceBuildTriggerModel = (*customTSourceBuildTriggerModel)(nil)

type (
	// TSourceBuildTriggerModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTSourceBuildTriggerModel.
	TSourceBuildTriggerModel interface {
		tSourceBuildTriggerModel
	}

	customTSourceBuildTriggerModel struct {
		*defaultTSourceBuildTriggerModel
	}
)

// NewTSourceBuildTriggerModel returns a model for the database table.
func NewTSourceBuildTriggerModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TSourceBuildTriggerModel {
	return &customTSourceBuildTriggerModel{
		defaultTSourceBuildTriggerModel: newTSourceBuildTriggerModel(conn, c, opts...),
	}
}
