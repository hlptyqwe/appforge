package svc

import (
	"appforge/services/system/internal/config"
	"appforge/services/system/models"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/core/syncx"
)

type ServiceContext struct {
	Config            config.Config
	DB                sqlx.SqlConn
	Cache             cache.Cache
	UserModel         models.SysUserModel
	RoleModel         models.SysRoleModel
	MenuModel         models.SysMenuModel
	UserRoleModel     models.SysUserRoleModel
	RoleMenuModel     models.SysRoleMenuModel
	LoginLogModel     models.SysLoginLogModel
	OpLogModel        models.SysOpLogModel
	ConfigModel       models.SysConfigModel
	TenantMode        models.SysTenantModel
	TenantDomainModel models.SysTenantDomainModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	configModel := models.NewSysConfigModel(conn, c.CacheRedis)
	return &ServiceContext{
		Config:            c,
		DB:                conn,
		Cache:             cache.New(c.CacheRedis, syncx.NewSingleFlight(), cache.NewStat(""), redis.Nil),
		UserModel:         models.NewSysUserModel(conn, c.CacheRedis),
		RoleModel:         models.NewSysRoleModel(conn, c.CacheRedis),
		MenuModel:         models.NewSysMenuModel(conn, c.CacheRedis),
		UserRoleModel:     models.NewSysUserRoleModel(conn, c.CacheRedis),
		RoleMenuModel:     models.NewSysRoleMenuModel(conn, c.CacheRedis),
		LoginLogModel:     models.NewSysLoginLogModel(conn, c.CacheRedis),
		OpLogModel:        models.NewSysOpLogModel(conn, c.CacheRedis),
		ConfigModel:       configModel,
		TenantMode:        models.NewSysTenantModel(conn, c.CacheRedis),
		TenantDomainModel: models.NewSysTenantDomainModel(conn),
	}
}
