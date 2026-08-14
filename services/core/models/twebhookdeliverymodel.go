package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TWebhookDeliveryModel = (*customTWebhookDeliveryModel)(nil)

type (
	// TWebhookDeliveryModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTWebhookDeliveryModel.
	TWebhookDeliveryModel interface {
		tWebhookDeliveryModel
	}

	customTWebhookDeliveryModel struct {
		*defaultTWebhookDeliveryModel
	}
)

// NewTWebhookDeliveryModel returns a model for the database table.
func NewTWebhookDeliveryModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TWebhookDeliveryModel {
	return &customTWebhookDeliveryModel{
		defaultTWebhookDeliveryModel: newTWebhookDeliveryModel(conn, c, opts...),
	}
}
