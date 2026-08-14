package svc

import (
	"appforge/services/core/internal/config"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config                  config.Config
	DB                      sqlx.SqlConn
	ApplicationModel        models.TAppApplicationModel
	VersionModel            models.TAppVersionModel
	SigningConfigModel      models.TAppSigningConfigModel
	ChannelModel            models.TPromotionChannelModel
	BuildTaskModel          models.TBuildTaskModel
	InstallModel            models.TChannelInstallModel
	ChannelEventModel       models.TChannelEventModel
	StorageObjectModel      models.TStorageObjectModel
	BrandingProfileModel    models.TAppBrandingProfileModel
	BrandingPreflightModel  models.TBrandingPreflightModel
	WhiteLabelTemplateModel models.TWhiteLabelTemplateModel
	WhiteLabelRevisionModel models.TWhiteLabelTemplateRevisionModel
	WhiteLabelProductModel  models.TWhiteLabelProductModel
	PackageCertificateModel models.TPackageCertificateBindingModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	return &ServiceContext{
		Config:                  c,
		DB:                      conn,
		ApplicationModel:        models.NewTAppApplicationModel(conn, c.CacheRedis),
		VersionModel:            models.NewTAppVersionModel(conn, c.CacheRedis),
		SigningConfigModel:      models.NewTAppSigningConfigModel(conn, c.CacheRedis),
		ChannelModel:            models.NewTPromotionChannelModel(conn, c.CacheRedis),
		BuildTaskModel:          models.NewTBuildTaskModel(conn, c.CacheRedis),
		InstallModel:            models.NewTChannelInstallModel(conn, c.CacheRedis),
		ChannelEventModel:       models.NewTChannelEventModel(conn, c.CacheRedis),
		StorageObjectModel:      models.NewTStorageObjectModel(conn, c.CacheRedis),
		BrandingProfileModel:    models.NewTAppBrandingProfileModel(conn, c.CacheRedis),
		BrandingPreflightModel:  models.NewTBrandingPreflightModel(conn, c.CacheRedis),
		WhiteLabelTemplateModel: models.NewTWhiteLabelTemplateModel(conn, c.CacheRedis),
		WhiteLabelRevisionModel: models.NewTWhiteLabelTemplateRevisionModel(conn, c.CacheRedis),
		WhiteLabelProductModel:  models.NewTWhiteLabelProductModel(conn, c.CacheRedis),
		PackageCertificateModel: models.NewTPackageCertificateBindingModel(conn, c.CacheRedis),
	}
}
