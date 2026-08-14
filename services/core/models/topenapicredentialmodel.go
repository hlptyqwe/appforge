package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TOpenApiCredentialModel = (*customTOpenApiCredentialModel)(nil)

type (
	// TOpenApiCredentialModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTOpenApiCredentialModel.
	TOpenApiCredentialModel interface {
		tOpenApiCredentialModel
	}

	customTOpenApiCredentialModel struct {
		*defaultTOpenApiCredentialModel
	}
)

// NewTOpenApiCredentialModel returns a model for the database table.
func NewTOpenApiCredentialModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TOpenApiCredentialModel {
	return &customTOpenApiCredentialModel{
		defaultTOpenApiCredentialModel: newTOpenApiCredentialModel(conn, c, opts...),
	}
}
