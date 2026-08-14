package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TOpenApiIdempotencyModel = (*customTOpenApiIdempotencyModel)(nil)

type (
	// TOpenApiIdempotencyModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTOpenApiIdempotencyModel.
	TOpenApiIdempotencyModel interface {
		tOpenApiIdempotencyModel
	}

	customTOpenApiIdempotencyModel struct {
		*defaultTOpenApiIdempotencyModel
	}
)

// NewTOpenApiIdempotencyModel returns a model for the database table.
func NewTOpenApiIdempotencyModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TOpenApiIdempotencyModel {
	return &customTOpenApiIdempotencyModel{
		defaultTOpenApiIdempotencyModel: newTOpenApiIdempotencyModel(conn, c, opts...),
	}
}
