package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TPackageCertificateBindingModel = (*customTPackageCertificateBindingModel)(nil)

type (
	// TPackageCertificateBindingModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTPackageCertificateBindingModel.
	TPackageCertificateBindingModel interface {
		tPackageCertificateBindingModel
		WithSession(session sqlx.Session) TPackageCertificateBindingModel
	}

	customTPackageCertificateBindingModel struct {
		*defaultTPackageCertificateBindingModel
	}
)

// NewTPackageCertificateBindingModel returns a model for the database table.
func NewTPackageCertificateBindingModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TPackageCertificateBindingModel {
	return &customTPackageCertificateBindingModel{
		defaultTPackageCertificateBindingModel: newTPackageCertificateBindingModel(conn, c, opts...),
	}
}

// WithSession returns a package-certificate model bound to the supplied transaction.
func (m *customTPackageCertificateBindingModel) WithSession(session sqlx.Session) TPackageCertificateBindingModel {
	return &customTPackageCertificateBindingModel{
		defaultTPackageCertificateBindingModel: &defaultTPackageCertificateBindingModel{
			CachedConn: m.CachedConn.WithSession(session),
			table:      m.table,
		},
	}
}
