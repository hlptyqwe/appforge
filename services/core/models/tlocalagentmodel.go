package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TLocalAgentModel = (*customTLocalAgentModel)(nil)

type (
	// TLocalAgentModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTLocalAgentModel.
	TLocalAgentModel interface {
		tLocalAgentModel
	}

	customTLocalAgentModel struct {
		*defaultTLocalAgentModel
	}
)

// NewTLocalAgentModel returns a model for the database table.
func NewTLocalAgentModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TLocalAgentModel {
	return &customTLocalAgentModel{
		defaultTLocalAgentModel: newTLocalAgentModel(conn, c, opts...),
	}
}
