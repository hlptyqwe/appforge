package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"strings"
	"syscall"

	"appforge/common/etcd"
	"appforge/common/observability"
	"appforge/common/rpcauth"
	pb "appforge/proto/core"
	"appforge/services/core/internal/config"
	"appforge/services/core/internal/logic"
	coreServer "appforge/services/core/internal/server"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	endpoints   = flag.String("etcd", "127.0.0.1:2379", "etcd endpoints")
	configKey   = flag.String("config", "/appforge/core-rpc/config", "etcd config key")
	commonKey   = flag.String("common", "/appforge/common/config", "common config key")
	processRole = flag.String("role", "rpc", "process role: rpc, webhook-worker, billing-worker, enterprise-worker, or all")
)

func main() {
	flag.Parse()

	var c config.Config
	if err := etcd.LoadFromEtcdAndMerge(strings.Split(*endpoints, ","), []string{*commonKey, *configKey}, &c); err != nil {
		log.Fatal(err)
	}
	if err := observability.ApplyEnvironment(&c.RpcServerConf.ServiceConf); err != nil {
		log.Fatal(err)
	}

	ctx := svc.NewServiceContext(c)
	workerCtx, stopWorkers := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopWorkers()

	switch *processRole {
	case "webhook-worker":
		c.ServiceConf.MustSetUp()
		logic.NewWebhookWorker(ctx).Start(workerCtx)
		fmt.Println("Starting core webhook worker...")
		<-workerCtx.Done()
		return
	case "billing-worker":
		c.ServiceConf.MustSetUp()
		logic.NewBillingWorker(ctx).Start(workerCtx)
		fmt.Println("Starting core billing worker...")
		<-workerCtx.Done()
		return
	case "enterprise-worker":
		c.ServiceConf.MustSetUp()
		logic.NewEnterpriseWorker(ctx).Start(workerCtx)
		fmt.Println("Starting core enterprise worker...")
		<-workerCtx.Done()
		return
	case "all":
		logic.NewWebhookWorker(ctx).Start(workerCtx)
		logic.NewBillingWorker(ctx).Start(workerCtx)
		logic.NewEnterpriseWorker(ctx).Start(workerCtx)
	case "rpc":
	default:
		log.Fatalf("unsupported process role %q", *processRole)
	}

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
