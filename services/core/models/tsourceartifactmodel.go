package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TSourceArtifactModel = (*customTSourceArtifactModel)(nil)

type (
	// TSourceArtifactModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTSourceArtifactModel.
	TSourceArtifactModel interface {
		tSourceArtifactModel
		WithSession(session sqlx.Session) TSourceArtifactModel
	}

	customTSourceArtifactModel struct {
		*defaultTSourceArtifactModel
	}
)

// NewTSourceArtifactModel returns a model for the database table.
func NewTSourceArtifactModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TSourceArtifactModel {
	return &customTSourceArtifactModel{
		defaultTSourceArtifactModel: newTSourceArtifactModel(conn, c, opts...),
	}
}

func (m *customTSourceArtifactModel) WithSession(session sqlx.Session) TSourceArtifactModel {
	return &customTSourceArtifactModel{defaultTSourceArtifactModel: &defaultTSourceArtifactModel{CachedConn: m.CachedConn.WithSession(session), table: m.table}}
}
