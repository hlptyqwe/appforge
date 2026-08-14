// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/logicutil"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformLocalAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformLocalAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformLocalAgentLogic {
	return &GetPlatformLocalAgentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformLocalAgentLogic) GetPlatformLocalAgent(req *types.PlatformLocalAgentIdReq) (resp *types.PlatformLocalAgentResp, err error) {
	return logicutil.Proxy[types.PlatformLocalAgentResp](l.ctx, req, l.svcCtx.CoreCli.GetLocalAgent)
}
