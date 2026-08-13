// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"flag"
	"fmt"
	"strings"

	"appforge/admin-api/internal/config"
	"appforge/admin-api/internal/handler"
	"appforge/admin-api/internal/middleware"
	"appforge/admin-api/internal/svc"

	"github.com/zeromicro/go-zero/rest"

	"appforge/common/etcd"
	um "appforge/common/middleware"
	"appforge/common/utils"
	"appforge/common/validation"

	"github.com/zeromicro/go-zero/rest/httpx"
)

var (
	endpoints = flag.String("etcd", "127.0.0.1:2379", "etcd endpoints")
	configKey = flag.String("config", "/appforge/admin-api/config", "etcd config key")
	commonKey = flag.String("common", "/appforge/common/config", "etcd common config key")
)

func main() {
	flag.Parse()
	httpx.SetValidator(validation.New())

	var c config.Config
	// 用 etcd 配置中心
	if err := etcd.LoadFromEtcdAndMerge(strings.Split(*endpoints, ","), []string{*commonKey, *configKey}, &c); err != nil {
		panic(err)
	}
	c.Middlewares.Log = false

	server := rest.MustNewServer(
		c.RestConf,
		rest.WithCors("*"),
		rest.WithCorsHeaders(string(utils.CtxKeyTenantId)),
	)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	requestLogMiddleware := um.NewRequestLogMiddleware("ADMIN-API")
	server.Use(requestLogMiddleware.Handle)
	auditMiddleware, err := middleware.NewAuditMiddleware(ctx.SystemCli, c.Audit.Routes)
	if err != nil {
		panic(fmt.Sprintf("invalid audit routes: %v", err))
	}
	server.Use(auditMiddleware.Handle)
	rbacMiddleware := middleware.NewRbacMiddleware(ctx)
	server.Use(rbacMiddleware.Handle)

	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
