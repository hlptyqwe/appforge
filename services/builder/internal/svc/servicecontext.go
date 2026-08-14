package svc

import (
	"fmt"

	"appforge/common/rpcauth"
	"appforge/common/secretbox"
	"appforge/common/storage"
	"appforge/proto/core"
	"appforge/services/builder/internal/config"

	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config     config.Config
	CoreClient core.CoreClient
	Store      storage.ObjectStore
	Secrets    *secretbox.Box
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
	return &ServiceContext{
		Config: c, CoreClient: core.NewCoreClient(coreRPC.Conn()), Store: store, Secrets: secrets,
	}
}
