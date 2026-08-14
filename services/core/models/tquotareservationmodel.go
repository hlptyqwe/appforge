package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TQuotaReservationModel = (*customTQuotaReservationModel)(nil)

type (
	// TQuotaReservationModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTQuotaReservationModel.
	TQuotaReservationModel interface {
		tQuotaReservationModel
	}

	customTQuotaReservationModel struct {
		*defaultTQuotaReservationModel
	}
)

// NewTQuotaReservationModel returns a model for the database table.
func NewTQuotaReservationModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TQuotaReservationModel {
	return &customTQuotaReservationModel{
		defaultTQuotaReservationModel: newTQuotaReservationModel(conn, c, opts...),
	}
}
