package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TAppSigningConfigModel = (*customTAppSigningConfigModel)(nil)

type (
	// TAppSigningConfigModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTAppSigningConfigModel.
	TAppSigningConfigModel interface {
		tAppSigningConfigModel
	}

	customTAppSigningConfigModel struct {
		*defaultTAppSigningConfigModel
	}
)

// NewTAppSigningConfigModel returns a model for the database table.
func NewTAppSigningConfigModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TAppSigningConfigModel {
	return &customTAppSigningConfigModel{
		defaultTAppSigningConfigModel: newTAppSigningConfigModel(conn, c, opts...),
	}
}
