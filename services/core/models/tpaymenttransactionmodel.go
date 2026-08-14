package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TPaymentTransactionModel = (*customTPaymentTransactionModel)(nil)

type (
	// TPaymentTransactionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTPaymentTransactionModel.
	TPaymentTransactionModel interface {
		tPaymentTransactionModel
	}

	customTPaymentTransactionModel struct {
		*defaultTPaymentTransactionModel
	}
)

// NewTPaymentTransactionModel returns a model for the database table.
func NewTPaymentTransactionModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TPaymentTransactionModel {
	return &customTPaymentTransactionModel{
		defaultTPaymentTransactionModel: newTPaymentTransactionModel(conn, c, opts...),
	}
}
