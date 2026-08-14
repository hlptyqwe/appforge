package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TUsageThresholdNotificationModel = (*customTUsageThresholdNotificationModel)(nil)

type (
	// TUsageThresholdNotificationModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTUsageThresholdNotificationModel.
	TUsageThresholdNotificationModel interface {
		tUsageThresholdNotificationModel
	}

	customTUsageThresholdNotificationModel struct {
		*defaultTUsageThresholdNotificationModel
	}
)

// NewTUsageThresholdNotificationModel returns a model for the database table.
func NewTUsageThresholdNotificationModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TUsageThresholdNotificationModel {
	return &customTUsageThresholdNotificationModel{
		defaultTUsageThresholdNotificationModel: newTUsageThresholdNotificationModel(conn, c, opts...),
	}
}
