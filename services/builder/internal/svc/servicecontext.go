package svc

import (
	"appforge/proto/core"
	"appforge/services/builder/internal/config"

	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config     config.Config
	CoreClient core.CoreClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	coreRPC := zrpc.MustNewClient(c.CoreRpc)
	return &ServiceContext{Config: c, CoreClient: core.NewCoreClient(coreRPC.Conn())}
}
