package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"appforge/common/etcd"
	"appforge/common/rpcauth"
	pb "appforge/proto/core"
	"appforge/services/core/internal/config"
	coreServer "appforge/services/core/internal/server"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	endpoints = flag.String("etcd", "127.0.0.1:2379", "etcd endpoints")
	configKey = flag.String("config", "/appforge/core-rpc/config", "etcd config key")
	commonKey = flag.String("common", "/appforge/common/config", "common config key")
)

func main() {
	flag.Parse()

	var c config.Config
	if err := etcd.LoadFromEtcdAndMerge(strings.Split(*endpoints, ","), []string{*commonKey, *configKey}, &c); err != nil {
		log.Fatal(err)
	}

	ctx := svc.NewServiceContext(c)
	if err := rpcauth.ValidateToken(c.InternalRpc.Token); err != nil {
		log.Fatal(err)
	}
	server := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterCoreServer(grpcServer, coreServer.NewCoreServer(ctx))
		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	server.AddUnaryInterceptors(rpcauth.UnaryServerInterceptor(c.InternalRpc.Token))
	server.AddStreamInterceptors(rpcauth.StreamServerInterceptor(c.InternalRpc.Token))
	defer server.Stop()

	fmt.Printf("Starting core rpc server at %s...\n", c.ListenOn)
	server.Start()
}
