package svc

import (
	"context"
	"fmt"

	"appforge/common/rpcauth"
	"appforge/common/secretbox"
	"appforge/common/secretprovider"
	"appforge/common/storage"
	"appforge/proto/core"
	"appforge/services/builder/internal/config"
	"appforge/services/builder/internal/secretresolver"

	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config     config.Config
	CoreClient core.CoreClient
	Store      storage.ObjectStore
	Secrets    *secretbox.Box
	Resolver   *secretprovider.Resolver
}

func NewServiceContext(c config.Config) *ServiceContext {
	if err := rpcauth.ValidateToken(c.InternalRpc.Token); err != nil {
		panic(err)
	}
	coreRPC := zrpc.MustNewClient(c.CoreRpc, zrpc.WithUnaryClientInterceptor(rpcauth.UnaryClientInterceptor(c.InternalRpc.Token)))
	store, err := storage.NewObjectStore(c.ObjectStorage.StorageConfig())
	if err != nil {
		panic(fmt.Sprintf("initialize builder object storage: %v", err))
	}
	secrets, err := secretbox.New(c.SigningSecrets.MasterKeyBase64)
	if err != nil {
		panic(fmt.Sprintf("initialize builder signing secrets: %v", err))
	}
	resolver, err := secretresolver.New(context.Background(), c)
	if err != nil {
		panic(err)
	}
	return &ServiceContext{
		Config: c, CoreClient: core.NewCoreClient(coreRPC.Conn()), Store: store, Secrets: secrets, Resolver: resolver,
	}
}
