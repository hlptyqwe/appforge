package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TWebhookEndpointModel = (*customTWebhookEndpointModel)(nil)

type (
	// TWebhookEndpointModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTWebhookEndpointModel.
	TWebhookEndpointModel interface {
		tWebhookEndpointModel
	}

	customTWebhookEndpointModel struct {
		*defaultTWebhookEndpointModel
	}
)

// NewTWebhookEndpointModel returns a model for the database table.
func NewTWebhookEndpointModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TWebhookEndpointModel {
	return &customTWebhookEndpointModel{
		defaultTWebhookEndpointModel: newTWebhookEndpointModel(conn, c, opts...),
	}
}
