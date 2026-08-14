package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TOutboxEventModel = (*customTOutboxEventModel)(nil)

type (
	// TOutboxEventModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTOutboxEventModel.
	TOutboxEventModel interface {
		tOutboxEventModel
	}

	customTOutboxEventModel struct {
		*defaultTOutboxEventModel
	}
)

// NewTOutboxEventModel returns a model for the database table.
func NewTOutboxEventModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TOutboxEventModel {
	return &customTOutboxEventModel{
		defaultTOutboxEventModel: newTOutboxEventModel(conn, c, opts...),
	}
}
