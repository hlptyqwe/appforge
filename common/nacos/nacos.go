package nacos

import (
	config "appforge/common/nacos/config"
	"appforge/common/nacos/naming"
	"appforge/common/nacos/types"
)

func NewRegistry(c types.NacosConf) (*naming.Registry, error)   { return naming.NewRegistry(c) }
func NewDiscovery(c types.NacosConf) (*naming.Discovery, error) { return naming.NewDiscovery(c) }

func RegisterGrpcResolver(c types.NacosConf) error { return naming.RegisterResolver(c) }

func NewConfigSubscriber(c types.NacosConf) (*config.Subscriber, error) {
	return config.NewSubscriber(c)
}

func NewConfigLoader(c types.NacosConf) (*config.Loader, error) {
	return config.NewLoader(c)
}
