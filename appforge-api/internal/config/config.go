// Code scaffolded by goctl. Safe to edit.

package config

import (
	"appforge/common/rpcauth"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Jwt struct {
		AccessSecret string
		AccessExpire int64
	} `json:"Jwt" yaml:"Jwt"`
	Audit          AuditConfig
	InternalRpc    rpcauth.Config
	SystemRpc      zrpc.RpcClientConf
	CoreRpc        zrpc.RpcClientConf
	BuilderRpc     zrpc.RpcClientConf
	SigningSecrets struct {
		MasterKeyBase64 string
	} `json:"SigningSecrets" yaml:"SigningSecrets"`
}

type AuditConfig struct {
	Routes []AuditRoute `json:"Routes" yaml:"Routes"`
}

type AuditRoute struct {
	Method string `json:"Method" yaml:"Method"`
	Path   string `json:"Path" yaml:"Path"`
}
