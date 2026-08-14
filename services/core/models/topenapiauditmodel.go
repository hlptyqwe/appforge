package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TOpenApiAuditModel = (*customTOpenApiAuditModel)(nil)

type (
	// TOpenApiAuditModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTOpenApiAuditModel.
	TOpenApiAuditModel interface {
		tOpenApiAuditModel
	}

	customTOpenApiAuditModel struct {
		*defaultTOpenApiAuditModel
	}
)

// NewTOpenApiAuditModel returns a model for the database table.
func NewTOpenApiAuditModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TOpenApiAuditModel {
	return &customTOpenApiAuditModel{
		defaultTOpenApiAuditModel: newTOpenApiAuditModel(conn, c, opts...),
	}
}
