package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SysOpLogModel = (*customSysOpLogModel)(nil)

type (
	// SysOpLogModel is an interface to be customized, add more methods here,
	// and implement the added methods in customSysOpLogModel.
	SysOpLogModel interface {
		sysOpLogModel
	}

	customSysOpLogModel struct {
		*defaultSysOpLogModel
	}
)

// NewSysOpLogModel returns a model for the database table.
func NewSysOpLogModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SysOpLogModel {
	return &customSysOpLogModel{
		defaultSysOpLogModel: newSysOpLogModel(conn, c, opts...),
	}
}
