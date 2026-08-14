package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TSourceWebhookEventModel = (*customTSourceWebhookEventModel)(nil)

type (
	// TSourceWebhookEventModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTSourceWebhookEventModel.
	TSourceWebhookEventModel interface {
		tSourceWebhookEventModel
	}

	customTSourceWebhookEventModel struct {
		*defaultTSourceWebhookEventModel
	}
)

// NewTSourceWebhookEventModel returns a model for the database table.
func NewTSourceWebhookEventModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TSourceWebhookEventModel {
	return &customTSourceWebhookEventModel{
		defaultTSourceWebhookEventModel: newTSourceWebhookEventModel(conn, c, opts...),
	}
}
