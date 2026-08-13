package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"appforge/common/etcd"
	pb "appforge/proto/builder"
	"appforge/services/builder/internal/config"
	builderServer "appforge/services/builder/internal/server"
	"appforge/services/builder/internal/svc"

	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	endpoints = flag.String("etcd", "127.0.0.1:2379", "etcd endpoints")
	configKey = flag.String("config", "/appforge/builder-rpc/config", "etcd config key")
	commonKey = flag.String("common", "/appforge/common/config", "common config key")
)

func main() {
	flag.Parse()

	var c config.Config
	if err := etcd.LoadFromEtcdAndMerge(strings.Split(*endpoints, ","), []string{*commonKey, *configKey}, &c); err != nil {
		log.Fatal(err)
	}

	ctx := svc.NewServiceContext(c)
	server := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterBuilderServer(grpcServer, builderServer.NewBuilderServer(ctx))
		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer server.Stop()

	fmt.Printf("Starting builder rpc server at %s...\n", c.ListenOn)
	server.Start()
}
