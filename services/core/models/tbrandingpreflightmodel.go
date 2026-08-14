package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TBrandingPreflightModel = (*customTBrandingPreflightModel)(nil)

type (
	// TBrandingPreflightModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTBrandingPreflightModel.
	TBrandingPreflightModel interface {
		tBrandingPreflightModel
	}

	customTBrandingPreflightModel struct {
		*defaultTBrandingPreflightModel
	}
)

// NewTBrandingPreflightModel returns a model for the database table.
func NewTBrandingPreflightModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TBrandingPreflightModel {
	return &customTBrandingPreflightModel{
		defaultTBrandingPreflightModel: newTBrandingPreflightModel(conn, c, opts...),
	}
}
