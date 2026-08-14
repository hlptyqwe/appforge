package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TInvoiceItemModel = (*customTInvoiceItemModel)(nil)

type (
	// TInvoiceItemModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTInvoiceItemModel.
	TInvoiceItemModel interface {
		tInvoiceItemModel
	}

	customTInvoiceItemModel struct {
		*defaultTInvoiceItemModel
	}
)

// NewTInvoiceItemModel returns a model for the database table.
func NewTInvoiceItemModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TInvoiceItemModel {
	return &customTInvoiceItemModel{
		defaultTInvoiceItemModel: newTInvoiceItemModel(conn, c, opts...),
	}
}
