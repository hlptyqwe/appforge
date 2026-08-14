package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TLocalAgentRegistrationModel = (*customTLocalAgentRegistrationModel)(nil)

type (
	// TLocalAgentRegistrationModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTLocalAgentRegistrationModel.
	TLocalAgentRegistrationModel interface {
		tLocalAgentRegistrationModel
	}

	customTLocalAgentRegistrationModel struct {
		*defaultTLocalAgentRegistrationModel
	}
)

// NewTLocalAgentRegistrationModel returns a model for the database table.
func NewTLocalAgentRegistrationModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TLocalAgentRegistrationModel {
	return &customTLocalAgentRegistrationModel{
		defaultTLocalAgentRegistrationModel: newTLocalAgentRegistrationModel(conn, c, opts...),
	}
}
