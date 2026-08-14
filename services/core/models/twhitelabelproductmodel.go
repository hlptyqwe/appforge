package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TWhiteLabelProductModel = (*customTWhiteLabelProductModel)(nil)

type (
	// TWhiteLabelProductModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTWhiteLabelProductModel.
	TWhiteLabelProductModel interface {
		tWhiteLabelProductModel
	}

	customTWhiteLabelProductModel struct {
		*defaultTWhiteLabelProductModel
	}
)

// NewTWhiteLabelProductModel returns a model for the database table.
func NewTWhiteLabelProductModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TWhiteLabelProductModel {
	return &customTWhiteLabelProductModel{
		defaultTWhiteLabelProductModel: newTWhiteLabelProductModel(conn, c, opts...),
	}
}
