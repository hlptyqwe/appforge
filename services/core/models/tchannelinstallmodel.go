package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TChannelInstallModel = (*customTChannelInstallModel)(nil)

type (
	// TChannelInstallModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTChannelInstallModel.
	TChannelInstallModel interface {
		tChannelInstallModel
	}

	customTChannelInstallModel struct {
		*defaultTChannelInstallModel
	}
)

// NewTChannelInstallModel returns a model for the database table.
func NewTChannelInstallModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TChannelInstallModel {
	return &customTChannelInstallModel{
		defaultTChannelInstallModel: newTChannelInstallModel(conn, c, opts...),
	}
}
