package models

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TAppBrandingProfileModel = (*customTAppBrandingProfileModel)(nil)

type (
	// TAppBrandingProfileModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTAppBrandingProfileModel.
	TAppBrandingProfileModel interface {
		tAppBrandingProfileModel
	}

	customTAppBrandingProfileModel struct {
		*defaultTAppBrandingProfileModel
	}
)

// NewTAppBrandingProfileModel returns a model for the database table.
func NewTAppBrandingProfileModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TAppBrandingProfileModel {
	return &customTAppBrandingProfileModel{
		defaultTAppBrandingProfileModel: newTAppBrandingProfileModel(conn, c, opts...),
	}
}
