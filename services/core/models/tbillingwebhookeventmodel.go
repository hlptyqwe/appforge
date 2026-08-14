package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TBillingWebhookEventModel = (*customTBillingWebhookEventModel)(nil)

type (
	// TBillingWebhookEventModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTBillingWebhookEventModel.
	TBillingWebhookEventModel interface {
		tBillingWebhookEventModel
	}

	customTBillingWebhookEventModel struct {
		*defaultTBillingWebhookEventModel
	}
)

// NewTBillingWebhookEventModel returns a model for the database table.
func NewTBillingWebhookEventModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TBillingWebhookEventModel {
	return &customTBillingWebhookEventModel{
		defaultTBillingWebhookEventModel: newTBillingWebhookEventModel(conn, c, opts...),
	}
}
