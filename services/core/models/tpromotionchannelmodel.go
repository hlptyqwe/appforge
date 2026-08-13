package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TPromotionChannelModel = (*customTPromotionChannelModel)(nil)

type (
	// TPromotionChannelModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTPromotionChannelModel.
	TPromotionChannelModel interface {
		tPromotionChannelModel
	}

	customTPromotionChannelModel struct {
		*defaultTPromotionChannelModel
	}
)

// NewTPromotionChannelModel returns a model for the database table.
func NewTPromotionChannelModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TPromotionChannelModel {
	return &customTPromotionChannelModel{
		defaultTPromotionChannelModel: newTPromotionChannelModel(conn, c, opts...),
	}
}
