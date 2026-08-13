package naming

import (
	"context"
	"net/url"
	"strings"
	"sync"

	"appforge/common/nacos/types"

	"google.golang.org/grpc/resolver"
)

const Scheme = "nacos"

// RegisterResolver 在进程启动时调用一次即可（admin-api / 任何 client 侧）
func RegisterResolver(c types.NacosConf) error {
	d, err := NewDiscovery(c)
	if err != nil {
		return err
	}
	resolver.Register(&nacosBuilder{d: d})
	return nil
}

type nacosBuilder struct {
	d *Discovery
}

func (b *nacosBuilder) Scheme() string { return Scheme }

// target: nacos:///system.rpc?group=DEFAULT_GROUP&cluster=DEFAULT
func (b *nacosBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	serviceName := strings.TrimPrefix(target.URL.Path, "/")
	q := target.URL.Query()

	// 允许在 Target 里覆盖 group/cluster
	if g := q.Get("group"); g != "" {
		b.d.c.Group = g
	}
	if cl := q.Get("cluster"); cl != "" {
		b.d.c.Cluster = cl
	}

	r := &nacosResolver{
		cc:          cc,
		serviceName: serviceName,
		d:           b.d,
	}
	if err := r.start(); err != nil {
		return nil, err
	}
	return r, nil
}

type nacosResolver struct {
	cc          resolver.ClientConn
	serviceName string
	d           *Discovery

	ctx    context.Context
	cancel context.CancelFunc

	mu    sync.Mutex
	addrs []string
}

func (r *nacosResolver) start() error {
	r.ctx, r.cancel = context.WithCancel(context.Background())

	// 1) 初次拉取
	addrs, err := r.d.Instances(r.serviceName)
	if err != nil {
		return err
	}
	r.update(addrs)

	// 2) watch 更新
	_ = r.d.Watch(r.serviceName, func(a []string) {
		r.update(a)
	})
	return nil
}

func (r *nacosResolver) ResolveNow(resolver.ResolveNowOptions) {}

func (r *nacosResolver) Close() {
	if r.cancel != nil {
		r.cancel()
	}
}

func (r *nacosResolver) update(addrs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.addrs = addrs
	state := resolver.State{
		Addresses: make([]resolver.Address, 0, len(addrs)),
	}
	for _, a := range addrs {
		state.Addresses = append(state.Addresses, resolver.Address{Addr: a})
	}
	_ = r.cc.UpdateState(state)
}

// 兼容旧 gRPC target 解析（某些版本 target.URL 可能为空）
func parseTarget(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
