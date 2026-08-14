// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"appforge/admin-api/internal/config"
	"appforge/admin-api/internal/handler"
	"appforge/admin-api/internal/middleware"
	"appforge/admin-api/internal/sourceworker"
	"appforge/admin-api/internal/svc"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"

	"appforge/common/etcd"
	um "appforge/common/middleware"
	"appforge/common/siem"
	"appforge/common/utils"
	"appforge/common/validation"
)

var (
	endpoints   = flag.String("etcd", "127.0.0.1:2379", "etcd endpoints")
	configKey   = flag.String("config", "/appforge/admin-api/config", "etcd config key")
	commonKey   = flag.String("common", "/appforge/common/config", "etcd common config key")
	processRole = flag.String("role", "api", "process role: api, source-trigger-worker, or all")
)

func main() {
	flag.Parse()
	httpx.SetValidator(validation.New())
	httpx.SetErrorHandlerCtx(func(ctx context.Context, err error) (int, any) {
		if _, ok := middleware.OpenApiPrincipalFromContext(ctx); ok {
			return middleware.OpenApiLogicErrorResponse(err)
		}
		return middleware.DefaultLogicErrorResponse(err)
	})

	var c config.Config
	if err := etcd.LoadFromEtcdAndMerge(strings.Split(*endpoints, ","), []string{*commonKey, *configKey}, &c); err != nil {
		panic(err)
	}
	if endpoint := strings.TrimSpace(os.Getenv("APPFORGE_SIEM_ENDPOINT")); endpoint != "" {
		c.Audit.SIEM.Enabled = true
		c.Audit.SIEM.Endpoint = endpoint
		c.Audit.SIEM.BearerTokenFile = strings.TrimSpace(os.Getenv("APPFORGE_SIEM_TOKEN_FILE"))
		c.Audit.SIEM.CACertificate = strings.TrimSpace(os.Getenv("APPFORGE_SIEM_CA_FILE"))
	}
	c.Middlewares.Log = false
	svcCtx := svc.NewServiceContext(c)
	workerCtx, stopWorker := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopWorker()
	switch *processRole {
	case "source-trigger-worker":
		sourceworker.New(svcCtx).Start(workerCtx)
		fmt.Println("Starting source trigger worker...")
		<-workerCtx.Done()
		return
	case "all":
		sourceworker.New(svcCtx).Start(workerCtx)
	case "api":
	default:
		log.Fatalf("unsupported process role %q", *processRole)
	}

	server := rest.MustNewServer(
		c.RestConf,
		rest.WithCors("*"),
		rest.WithCorsHeaders(string(utils.CtxKeyTenantId)),
	)
	defer server.Stop()

	server.Use(um.NewRequestLogMiddleware("ADMIN-API").Handle)
	server.Use(middleware.NewOfflineLicenseMiddleware(svcCtx.License).Handle)
	server.Use(middleware.NewPublicRateLimitMiddleware().Handle)
	siemExporter, err := siem.New(c.Audit.SIEM)
	if err != nil {
		panic(fmt.Sprintf("initialize SIEM audit exporter: %v", err))
	}
	siemExporter.Start(workerCtx)
	auditMiddleware, err := middleware.NewAuditMiddleware(svcCtx.SystemCli, c.Audit.Routes, siemExporter)
	if err != nil {
		panic(fmt.Sprintf("invalid audit routes: %v", err))
	}
	server.Use(auditMiddleware.Handle)
	server.Use(middleware.NewOpenApiMiddleware(svcCtx).Handle)
	server.Use(middleware.NewRbacMiddleware(svcCtx).Handle)
	handler.RegisterHandlers(server, svcCtx)
	handler.RegisterAgentHandlers(server, svcCtx)
	handler.RegisterSourceOAuthHandlers(server, svcCtx)
	handler.RegisterBillingHandlers(server, svcCtx)
	server.AddRoute(rest.Route{Method: "GET", Path: "/healthz", Handler: handler.HealthHandler})
	server.AddRoute(rest.Route{Method: "GET", Path: "/readyz", Handler: handler.ReadyHandler(svcCtx)})
	server.AddRoute(rest.Route{Method: "POST", Path: "/public/v1/local-agent/register", Handler: handler.RegisterLocalAgentPublicHandler(svcCtx)})
	if err := handler.StartLocalAgentGateway(workerCtx, c.LocalAgentGateway, svcCtx); err != nil {
		panic(fmt.Sprintf("start Local Agent mTLS gateway: %v", err))
	}

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
