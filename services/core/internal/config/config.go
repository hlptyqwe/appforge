// Code scaffolded for the APK distribution core RPC.
package config

import (
	"appforge/common/rpcauth"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Mysql struct {
		DataSource string
	} `json:"Mysql" yaml:"Mysql"`
	CacheRedis  cache.CacheConf `json:"CacheRedis" yaml:"CacheRedis"`
	InternalRpc rpcauth.Config
}
