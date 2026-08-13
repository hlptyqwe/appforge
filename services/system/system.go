package main

import (
	"flag"
	"fmt"
	"strings"

	pb "appforge/proto/system"
	"appforge/services/system/internal/config"
	admin "appforge/services/system/internal/server/admin"
	app "appforge/services/system/internal/server/app"
	system "appforge/services/system/internal/server/system"
	"appforge/services/system/internal/svc"

	"appforge/common/etcd"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/mon"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	endpoints = flag.String("etcd", "127.0.0.1:2379", "etcd endpoints")
	configKey = flag.String("config", "/appforge/system-rpc/config", "etcd config key")
	commonKey = flag.String("common", "/appforge/common/config", "etcd common config key")
)

func main() {
	flag.Parse()

	var c config.Config

	// 用 etcd 配置中心
	if err := etcd.LoadFromEtcdAndMerge(strings.Split(*endpoints, ","), []string{*commonKey, *configKey}, &c); err != nil {
		panic(err)
	}

	logx.SetLevel(logx.ErrorLevel)
	mon.DisableInfoLog()

	ctx := svc.NewServiceContext(c)

	server := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterAdminServer(grpcServer, admin.NewAdminServer(ctx))
		pb.RegisterAppServer(grpcServer, app.NewAppServer(ctx))
		pb.RegisterSystemServer(grpcServer, system.NewSystemServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	defer server.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	server.Start()
}
