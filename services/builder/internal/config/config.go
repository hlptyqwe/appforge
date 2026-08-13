// Code scaffolded for the APK Builder RPC.
package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf
	CoreRpc zrpc.RpcClientConf `json:"CoreRpc" yaml:"CoreRpc"`
	Builder struct {
		Id           string
		LeaseSeconds int32
	} `json:"Builder" yaml:"Builder"`
}
