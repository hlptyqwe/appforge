// Code scaffolded by goctl. Safe to edit.

package svc

import (
	"context"
	"fmt"
	"strconv"

	"appforge/admin-api/internal/config"
	"appforge/common/utils"
	"appforge/proto/core"
	"appforge/proto/system"

	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ServiceContext struct {
	Config    config.Config
	SystemCli system.AdminClient
	CoreCli   core.CoreClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	metadataInterceptor := zrpc.WithUnaryClientInterceptor(func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		pairs := []string{utils.CtxKeySubjectDomain, utils.SubjectDomainSystem}
		if userID, err := utils.GetUserIdFromCtx(ctx); err == nil {
			pairs = append(pairs, utils.CtxKeyUid, strconv.FormatInt(userID, 10))
		}
		if username, err := utils.GetUsernameFromCtx(ctx); err == nil {
			pairs = append(pairs, utils.CtxKeyUsername, username)
		}
		if tenantID, err := utils.GetTenantIdFromCtx(ctx); err == nil {
			pairs = append(pairs, utils.CtxKeyTenantId, fmt.Sprintf("%d", tenantID))
		}
		if userType, err := utils.GetUserTypeFromCtx(ctx); err == nil {
			pairs = append(pairs, utils.CtxKeyUserType, strconv.FormatInt(userType, 10))
		}
		return invoker(metadata.AppendToOutgoingContext(ctx, pairs...), method, req, reply, cc, opts...)
	})

	systemClient := zrpc.MustNewClient(c.SystemRpc, metadataInterceptor)
	coreClient := zrpc.MustNewClient(c.CoreRpc, metadataInterceptor)
	return &ServiceContext{
		Config:    c,
		SystemCli: system.NewAdminClient(systemClient.Conn()),
		CoreCli:   core.NewCoreClient(coreClient.Conn()),
	}
}
