package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"strings"
	"syscall"

	"appforge/common/etcd"
	"appforge/common/rpcauth"
	builderpb "appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/config"
	"appforge/services/builder/internal/worker"

	"github.com/zeromicro/go-zero/zrpc"
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
	if err := rpcauth.ValidateToken(c.InternalRpc.Token); err != nil {
		log.Fatal(err)
	}
	clientAuth := zrpc.WithUnaryClientInterceptor(rpcauth.UnaryClientInterceptor(c.InternalRpc.Token))
	builderConnection := zrpc.MustNewClient(c.BuilderRpc, clientAuth)
	coreConnection := zrpc.MustNewClient(c.CoreRpc, clientAuth)
	runner, err := worker.New(c, builderpb.NewBuilderClient(builderConnection.Conn()), core.NewCoreClient(coreConnection.Conn()))
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Printf("starting APK builder worker %s", c.Builder.Id)
	if err := runner.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
