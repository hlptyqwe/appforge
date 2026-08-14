package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ THybridArtifactReferenceModel = (*customTHybridArtifactReferenceModel)(nil)

type (
	// THybridArtifactReferenceModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTHybridArtifactReferenceModel.
	THybridArtifactReferenceModel interface {
		tHybridArtifactReferenceModel
	}

	customTHybridArtifactReferenceModel struct {
		*defaultTHybridArtifactReferenceModel
	}
)

// NewTHybridArtifactReferenceModel returns a model for the database table.
func NewTHybridArtifactReferenceModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) THybridArtifactReferenceModel {
	return &customTHybridArtifactReferenceModel{
		defaultTHybridArtifactReferenceModel: newTHybridArtifactReferenceModel(conn, c, opts...),
	}
}
