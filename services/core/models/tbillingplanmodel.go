package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TBillingPlanModel = (*customTBillingPlanModel)(nil)

type (
	// TBillingPlanModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTBillingPlanModel.
	TBillingPlanModel interface {
		tBillingPlanModel
	}

	customTBillingPlanModel struct {
		*defaultTBillingPlanModel
	}
)

// NewTBillingPlanModel returns a model for the database table.
func NewTBillingPlanModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TBillingPlanModel {
	return &customTBillingPlanModel{
		defaultTBillingPlanModel: newTBillingPlanModel(conn, c, opts...),
	}
}
