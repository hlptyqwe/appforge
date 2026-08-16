package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TAirGappedPackageModel = (*customTAirGappedPackageModel)(nil)

type (
	// TAirGappedPackageModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTAirGappedPackageModel.
	TAirGappedPackageModel interface {
		tAirGappedPackageModel
		WithSession(session sqlx.Session) TAirGappedPackageModel
	}

	customTAirGappedPackageModel struct {
		*defaultTAirGappedPackageModel
	}
)

// NewTAirGappedPackageModel returns a model for the database table.
func NewTAirGappedPackageModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TAirGappedPackageModel {
	return &customTAirGappedPackageModel{
		defaultTAirGappedPackageModel: newTAirGappedPackageModel(conn, c, opts...),
	}
}

// WithSession returns an AIR_GAPPED package model bound to the transaction.
func (m *customTAirGappedPackageModel) WithSession(session sqlx.Session) TAirGappedPackageModel {
	return &customTAirGappedPackageModel{defaultTAirGappedPackageModel: &defaultTAirGappedPackageModel{CachedConn: m.CachedConn.WithSession(session), table: m.table}}
}
