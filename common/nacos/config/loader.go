package config

import (
	"fmt"
	"appforge/common/nacos/naming"
	"appforge/common/nacos/types"

	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/zeromicro/go-zero/core/conf"
	"gopkg.in/yaml.v3"
)

type Loader struct {
	c   types.NacosConf
	cli config_client.IConfigClient
}

func NewLoader(c types.NacosConf) (*Loader, error) {
	cli, err := naming.NewConfigClient(c)
	if err != nil {
		return nil, err
	}
	if c.Group == "" {
		c.Group = "DEFAULT_GROUP"
	}
	return &Loader{c: c, cli: cli}, nil
}

func (l *Loader) Load(group string, dataId string) (string, error) {
	if dataId == "" {
		return "", fmt.Errorf("nacos DataId is empty")
	}
	content, err := l.cli.GetConfig(vo.ConfigParam{
		DataId: dataId,
		Group:  group,
	})
	if err != nil {
		return "", err
	}
	return content, nil
}

func (l *Loader) LoadMergedConfigFromNacos(group, commonId, serviceId string, out any) error {
	commonStr, err := l.Load(commonId, group)
	if err != nil {
		return err
	}
	serviceStr, err := l.Load(serviceId, group)
	if err != nil {
		return err
	}

	var m1 map[string]any
	var m2 map[string]any
	if err := yaml.Unmarshal([]byte(commonStr), &m1); err != nil {
		return err
	}
	if err := yaml.Unmarshal([]byte(serviceStr), &m2); err != nil {
		return err
	}

	merged := deepMerge(m1, m2) // m2 覆盖 m1
	bs, err := yaml.Marshal(merged)
	if err != nil {
		return err
	}

	return conf.LoadFromYamlBytes(bs, out)
}

func deepMerge(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for k, v := range src {
		if vMap, ok := v.(map[string]any); ok {
			if dMap, ok := dst[k].(map[string]any); ok {
				dst[k] = deepMerge(dMap, vMap)
			} else {
				dst[k] = deepMerge(map[string]any{}, vMap)
			}
			continue
		}
		dst[k] = v
	}
	return dst
}
