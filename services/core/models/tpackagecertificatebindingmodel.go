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
