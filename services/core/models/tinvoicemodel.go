package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TInvoiceModel = (*customTInvoiceModel)(nil)

type (
	// TInvoiceModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTInvoiceModel.
	TInvoiceModel interface {
		tInvoiceModel
	}

	customTInvoiceModel struct {
		*defaultTInvoiceModel
	}
)

// NewTInvoiceModel returns a model for the database table.
func NewTInvoiceModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TInvoiceModel {
	return &customTInvoiceModel{
		defaultTInvoiceModel: newTInvoiceModel(conn, c, opts...),
	}
}
