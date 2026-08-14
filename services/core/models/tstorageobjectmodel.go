package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TStorageObjectModel = (*customTStorageObjectModel)(nil)

type (
	// TStorageObjectModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTStorageObjectModel.
	TStorageObjectModel interface {
		tStorageObjectModel
		WithSession(session sqlx.Session) TStorageObjectModel
	}

	customTStorageObjectModel struct {
		*defaultTStorageObjectModel
	}
)

// NewTStorageObjectModel returns a model for the database table.
func NewTStorageObjectModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TStorageObjectModel {
	return &customTStorageObjectModel{
		defaultTStorageObjectModel: newTStorageObjectModel(conn, c, opts...),
	}
}

func (m *customTStorageObjectModel) WithSession(session sqlx.Session) TStorageObjectModel {
	return &customTStorageObjectModel{defaultTStorageObjectModel: &defaultTStorageObjectModel{CachedConn: m.CachedConn.WithSession(session), table: m.table}}
}
