package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TWhiteLabelTemplateRevisionModel = (*customTWhiteLabelTemplateRevisionModel)(nil)

type (
	// TWhiteLabelTemplateRevisionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTWhiteLabelTemplateRevisionModel.
	TWhiteLabelTemplateRevisionModel interface {
		tWhiteLabelTemplateRevisionModel
	}

	customTWhiteLabelTemplateRevisionModel struct {
		*defaultTWhiteLabelTemplateRevisionModel
	}
)

// NewTWhiteLabelTemplateRevisionModel returns a model for the database table.
func NewTWhiteLabelTemplateRevisionModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TWhiteLabelTemplateRevisionModel {
	return &customTWhiteLabelTemplateRevisionModel{
		defaultTWhiteLabelTemplateRevisionModel: newTWhiteLabelTemplateRevisionModel(conn, c, opts...),
	}
}
