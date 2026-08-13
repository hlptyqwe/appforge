package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TChannelEventModel = (*customTChannelEventModel)(nil)

type (
	// TChannelEventModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTChannelEventModel.
	TChannelEventModel interface {
		tChannelEventModel
	}

	customTChannelEventModel struct {
		*defaultTChannelEventModel
	}
)

// NewTChannelEventModel returns a model for the database table.
func NewTChannelEventModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TChannelEventModel {
	return &customTChannelEventModel{
		defaultTChannelEventModel: newTChannelEventModel(conn, c, opts...),
	}
}
