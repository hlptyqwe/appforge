package etcd

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	v3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v2"
)

func LoadFromEtcdAndMerge(hosts []string, keys []string, c any) error {
	cli, err := v3.New(v3.Config{
		Endpoints:   hosts,
		DialTimeout: 3 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to connect etcd: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	merged := make(map[string]any)

	for _, key := range keys {
		resp, err := cli.Get(ctx, key)
		if err != nil || len(resp.Kvs) == 0 {
			return fmt.Errorf("failed to get config from etcd key=%s err=%w", key, err)
		}

		data := resp.Kvs[0].Value

		var m map[string]any
		if err := yaml.Unmarshal(data, &m); err != nil {
			logx.Errorf("yaml parse failed key=%s err=%v", key, err)
			return fmt.Errorf("failed to parse yaml key=%s err=%w", key, err)
		}

		deepMerge(merged, m)
	}

	// 最后一次性 decode 到 struct
	bs, _ := yaml.Marshal(merged)
	if err := conf.LoadFromYamlBytes(bs, c); err != nil {
		logx.Errorf("load merged yaml failed err=%v yaml=%s", err, string(bs))
		return fmt.Errorf("failed to load merged yaml err=%w", err)
	}
	return nil
}

func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if vMap, ok := v.(map[string]any); ok {
			if dstMap, ok2 := dst[k].(map[string]any); ok2 {
				deepMerge(dstMap, vMap)
			} else {
				dst[k] = vMap
			}
		} else {
			dst[k] = v
		}
	}
}

func WatcherConfig[T any](
	hosts []string,
	key string,
	listeners ...func(T),
) {
	go func() {
		cli, err := v3.New(v3.Config{
			Endpoints:   hosts,
			DialTimeout: 3 * time.Second,
		})
		if err != nil {
			logx.Errorf("create etcd watcher failed key=%s err=%v", key, err)
			return
		}
		defer cli.Close()

		watchChan := cli.Watch(context.Background(), key)

		for response := range watchChan {
			if response.Err() != nil {
				logx.Errorf(
					"watch config failed key=%s err=%v",
					key,
					response.Err(),
				)
				continue
			}

			for _, event := range response.Events {
				if event.Type != v3.EventTypePut ||
					len(event.Kv.Value) == 0 {
					continue
				}

				var value T
				if err := conf.LoadFromYamlBytes(event.Kv.Value, &value); err != nil {
					logx.Errorf(
						"parse watched config failed key=%s err=%v",
						key,
						err,
					)
					continue
				}

				for _, listener := range listeners {
					if listener != nil {
						listener(value)
					}
				}
			}
		}
	}()
}
