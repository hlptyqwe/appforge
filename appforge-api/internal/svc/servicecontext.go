// Code scaffolded by goctl. Safe to edit.

package svc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"appforge/admin-api/internal/config"
	"appforge/common/offlinelicense"
	"appforge/common/rpcauth"
	"appforge/common/secretbox"
	"appforge/common/utils"
	"appforge/proto/builder"
	"appforge/proto/core"
	"appforge/proto/system"

	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ServiceContext struct {
	Config     config.Config
	SystemCli  system.AdminClient
	CoreCli    core.CoreClient
	BuilderCli builder.BuilderClient
	Secrets    *secretbox.Box
	License    *offlinelicense.VerifiedLicense
}

func NewServiceContext(c config.Config) *ServiceContext {
	if err := rpcauth.ValidateToken(c.InternalRpc.Token); err != nil {
		panic(err)
	}
	var verifiedLicense *offlinelicense.VerifiedLicense
	if c.OfflineLicense.Enabled {
		var err error
		verifiedLicense, err = offlinelicense.VerifyFile(offlinelicense.Config{
			LicenseFile: c.OfflineLicense.LicenseFile, PublicKeyFile: c.OfflineLicense.PublicKeyFile,
			StateFile: c.OfflineLicense.StateFile, DeploymentID: c.OfflineLicense.DeploymentId,
			DeploymentMode: c.OfflineLicense.DeploymentMode, ClockRollbackTolerance: c.OfflineLicense.ClockRollbackTolerance,
		}, time.Now())
		if err != nil {
			panic(fmt.Sprintf("verify offline enterprise license: %v", err))
		}
	}
	metadataInterceptor := zrpc.WithUnaryClientInterceptor(func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		pairs := []string{
			utils.CtxKeySubjectDomain, utils.SubjectDomainSystem,
			rpcauth.MetadataKey, c.InternalRpc.Token,
		}
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
		if appScope, err := utils.GetAppScopeFromCtx(ctx); err == nil {
			pairs = append(pairs, utils.CtxKeyAppScope, strconv.FormatInt(appScope, 10))
		}
		return invoker(metadata.AppendToOutgoingContext(ctx, pairs...), method, req, reply, cc, opts...)
	})

	systemClient := zrpc.MustNewClient(c.SystemRpc, metadataInterceptor)
	coreClient := zrpc.MustNewClient(c.CoreRpc, metadataInterceptor)
	builderClient := zrpc.MustNewClient(c.BuilderRpc, metadataInterceptor)
	secrets, err := secretbox.New(c.SigningSecrets.MasterKeyBase64)
	if err != nil {
		panic(fmt.Sprintf("initialize signing secret encryption: %v", err))
	}
	return &ServiceContext{
		Config:     c,
		SystemCli:  system.NewAdminClient(systemClient.Conn()),
		CoreCli:    core.NewCoreClient(coreClient.Conn()),
		BuilderCli: builder.NewBuilderClient(builderClient.Conn()),
		Secrets:    secrets,
		License:    verifiedLicense,
	}
}
