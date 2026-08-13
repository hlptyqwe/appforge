package naming

import (
	"fmt"

	"appforge/common/nacos/types"

	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

type Registry struct {
	c      types.NacosConf
	client naming_client.INamingClient
}

func NewRegistry(c types.NacosConf) (*Registry, error) {
	cli, err := NewNamingClient(c)
	if err != nil {
		return nil, err
	}
	if c.Group == "" {
		c.Group = "DEFAULT_GROUP"
	}
	if c.Cluster == "" {
		c.Cluster = "DEFAULT"
	}
	return &Registry{c: c, client: cli}, nil
}

func (r *Registry) Register(serviceName, ip string, port uint64, meta map[string]string) error {
	ok, err := r.client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        port,
		ServiceName: serviceName,
		GroupName:   r.c.Group,
		ClusterName: r.c.Cluster,
		Weight:      1,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    meta,
	})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("nacos register instance failed: %s", serviceName)
	}
	return nil
}

func (r *Registry) Deregister(serviceName, ip string, port uint64) error {
	ok, err := r.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          ip,
		Port:        port,
		ServiceName: serviceName,
		GroupName:   r.c.Group,
		Cluster:     r.c.Cluster,
		Ephemeral:   true,
	})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("nacos deregister instance failed: %s", serviceName)
	}
	return nil
}
