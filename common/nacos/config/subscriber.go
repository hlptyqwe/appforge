package config

import (
	"fmt"
	"sync"

	"appforge/common/nacos/naming"
	"appforge/common/nacos/types"

	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

type Subscriber struct {
	c   types.NacosConf
	cli config_client.IConfigClient

	mu        sync.Mutex
	listeners []func()
}

func NewSubscriber(c types.NacosConf) (*Subscriber, error) {
	cli, err := naming.NewConfigClient(c)
	if err != nil {
		return nil, err
	}
	return &Subscriber{c: c, cli: cli}, nil
}

// Value 返回最新配置内容（yaml/json 字符串）
func (s *Subscriber) Value(group string, dataId string) (string, error) {
	if dataId == "" {
		return "", fmt.Errorf("nacos DataId is empty")
	}
	return s.cli.GetConfig(vo.ConfigParam{
		DataId: dataId,
		Group:  group,
	})
}

// AddListener 监听配置变更；变更时调用 go-zero configcenter 的回调
func (s *Subscriber) AddListener(group string, dataId string, listener func()) error {
	s.mu.Lock()
	s.listeners = append(s.listeners, listener)
	s.mu.Unlock()

	if dataId == "" {
		return fmt.Errorf("nacos DataId is empty")
	}
	return s.cli.ListenConfig(vo.ConfigParam{
		DataId: dataId,
		Group:  group,
		OnChange: func(_, _, _, _ string) {
			s.mu.Lock()
			ls := append([]func(){}, s.listeners...)
			s.mu.Unlock()
			for _, fn := range ls {
				fn()
			}
		},
	})
}
