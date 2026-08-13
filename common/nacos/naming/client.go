package naming

import (
	"fmt"
	"strings"
	"time"

	"appforge/common/nacos/types"

	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

func parseAddr(addr string) (ip string, port uint64, err error) {
	parts := strings.Split(strings.TrimSpace(addr), ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid nacos addr: %s", addr)
	}
	ip = parts[0]
	var p uint64
	_, err = fmt.Sscanf(parts[1], "%d", &p)
	if err != nil {
		return "", 0, fmt.Errorf("invalid nacos port: %s", addr)
	}
	return ip, p, nil
}

func serverConfigs(c types.NacosConf) ([]constant.ServerConfig, error) {
	ip, port, err := parseAddr(c.Addr)
	if err != nil {
		return nil, err
	}
	return []constant.ServerConfig{
		*constant.NewServerConfig(ip, port),
	}, nil
}

func clientConfig(c types.NacosConf) constant.ClientConfig {
	return *constant.NewClientConfig(
		constant.WithNamespaceId(c.Namespace),
		constant.WithTimeoutMs(6000),
		constant.WithNotLoadCacheAtStart(true),
		constant.WithLogDir("logs/nacos"),
		constant.WithCacheDir("cache/nacos"),
		constant.WithLogLevel("warn"),
		constant.WithUsername(c.Username),
		constant.WithPassword(c.Password),
		constant.WithBeatInterval(5*time.Second.Milliseconds()),
	)
}

func NewNamingClient(c types.NacosConf) (naming_client.INamingClient, error) {
	sc, err := serverConfigs(c)
	if err != nil {
		return nil, err
	}
	cc := clientConfig(c)
	return clients.NewNamingClient(voNacosParam(sc, cc))
}

func NewConfigClient(c types.NacosConf) (config_client.IConfigClient, error) {
	sc, err := serverConfigs(c)
	if err != nil {
		return nil, err
	}
	cc := clientConfig(c)
	return clients.NewConfigClient(voNacosParam(sc, cc))
}

// nacos-sdk-go 的 vo.NacosClientParam
func voNacosParam(sc []constant.ServerConfig, cc constant.ClientConfig) vo.NacosClientParam {
	return vo.NacosClientParam{
		ClientConfig:  &cc,
		ServerConfigs: sc,
	}
}
