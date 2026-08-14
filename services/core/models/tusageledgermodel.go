package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TUsageLedgerModel = (*customTUsageLedgerModel)(nil)

type (
	// TUsageLedgerModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTUsageLedgerModel.
	TUsageLedgerModel interface {
		tUsageLedgerModel
	}

	customTUsageLedgerModel struct {
		*defaultTUsageLedgerModel
	}
)

// NewTUsageLedgerModel returns a model for the database table.
func NewTUsageLedgerModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TUsageLedgerModel {
	return &customTUsageLedgerModel{
		defaultTUsageLedgerModel: newTUsageLedgerModel(conn, c, opts...),
	}
}
