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

type ListPlatformLocalAgentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformLocalAgentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformLocalAgentsLogic {
	return &ListPlatformLocalAgentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformLocalAgentsLogic) ListPlatformLocalAgents(req *types.ListPlatformLocalAgentsReq) (resp *types.PlatformLocalAgentListResp, err error) {
	return logicutil.Proxy[types.PlatformLocalAgentListResp](l.ctx, req, l.svcCtx.CoreCli.ListLocalAgents)
}
