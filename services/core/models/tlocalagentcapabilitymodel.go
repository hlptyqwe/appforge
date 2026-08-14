package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TLocalAgentCapabilityModel = (*customTLocalAgentCapabilityModel)(nil)

type (
	// TLocalAgentCapabilityModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTLocalAgentCapabilityModel.
	TLocalAgentCapabilityModel interface {
		tLocalAgentCapabilityModel
	}

	customTLocalAgentCapabilityModel struct {
		*defaultTLocalAgentCapabilityModel
	}
)

// NewTLocalAgentCapabilityModel returns a model for the database table.
func NewTLocalAgentCapabilityModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TLocalAgentCapabilityModel {
	return &customTLocalAgentCapabilityModel{
		defaultTLocalAgentCapabilityModel: newTLocalAgentCapabilityModel(conn, c, opts...),
	}
}
