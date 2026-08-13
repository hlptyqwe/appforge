// Code scaffolded by goctl. Safe to edit.

package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Jwt struct {
		AccessSecret string
		AccessExpire int64
	} `json:"Jwt" yaml:"Jwt"`
	Audit     AuditConfig
	SystemRpc zrpc.RpcClientConf
	CoreRpc   zrpc.RpcClientConf
}

type AuditConfig struct {
	Routes []AuditRoute `json:"Routes" yaml:"Routes"`
}

type AuditRoute struct {
	Method string `json:"Method" yaml:"Method"`
	Path   string `json:"Path" yaml:"Path"`
}
