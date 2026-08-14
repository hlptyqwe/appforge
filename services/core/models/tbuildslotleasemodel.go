package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TBuildSlotLeaseModel = (*customTBuildSlotLeaseModel)(nil)

type (
	// TBuildSlotLeaseModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTBuildSlotLeaseModel.
	TBuildSlotLeaseModel interface {
		tBuildSlotLeaseModel
	}

	customTBuildSlotLeaseModel struct {
		*defaultTBuildSlotLeaseModel
	}
)

// NewTBuildSlotLeaseModel returns a model for the database table.
func NewTBuildSlotLeaseModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TBuildSlotLeaseModel {
	return &customTBuildSlotLeaseModel{
		defaultTBuildSlotLeaseModel: newTBuildSlotLeaseModel(conn, c, opts...),
	}
}
