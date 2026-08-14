package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TLocalAgentCertificateModel = (*customTLocalAgentCertificateModel)(nil)

type (
	// TLocalAgentCertificateModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTLocalAgentCertificateModel.
	TLocalAgentCertificateModel interface {
		tLocalAgentCertificateModel
	}

	customTLocalAgentCertificateModel struct {
		*defaultTLocalAgentCertificateModel
	}
)

// NewTLocalAgentCertificateModel returns a model for the database table.
func NewTLocalAgentCertificateModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TLocalAgentCertificateModel {
	return &customTLocalAgentCertificateModel{
		defaultTLocalAgentCertificateModel: newTLocalAgentCertificateModel(conn, c, opts...),
	}
}
