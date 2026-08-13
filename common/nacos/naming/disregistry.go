package naming

import (
	"fmt"
	"sync"

	"appforge/common/nacos/types"

	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

type Discovery struct {
	c      types.NacosConf
	client naming_client.INamingClient
}

func NewDiscovery(c types.NacosConf) (*Discovery, error) {
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
	return &Discovery{c: c, client: cli}, nil
}

func (d *Discovery) Instances(serviceName string) ([]string, error) {
	svcs, err := d.client.SelectInstances(vo.SelectInstancesParam{
		ServiceName: serviceName,
		GroupName:   d.c.Group,
		Clusters:    []string{d.c.Cluster},
		HealthyOnly: true,
	})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(svcs))
	for _, it := range svcs {
		out = append(out, fmt.Sprintf("%s:%d", it.Ip, it.Port))
	}
	return out, nil
}

func (d *Discovery) Watch(serviceName string, onChange func(addrs []string)) error {
	var mu sync.Mutex
	return d.client.Subscribe(&vo.SubscribeParam{
		ServiceName: serviceName,
		GroupName:   d.c.Group,
		Clusters:    []string{d.c.Cluster},
		SubscribeCallback: func(services []model.SubscribeService, err error) {
			if err != nil {
				return
			}
			addrs := make([]string, 0, len(services))
			for _, s := range services {
				if s.Healthy {
					addrs = append(addrs, fmt.Sprintf("%s:%d", s.Ip, s.Port))
				}
			}
			mu.Lock()
			onChange(addrs)
			mu.Unlock()
		},
	})
}
