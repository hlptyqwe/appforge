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

type DrainPlatformLocalAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDrainPlatformLocalAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DrainPlatformLocalAgentLogic {
	return &DrainPlatformLocalAgentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DrainPlatformLocalAgentLogic) DrainPlatformLocalAgent(req *types.DrainPlatformLocalAgentReq) (resp *types.PlatformLocalAgentResp, err error) {
	return logicutil.Proxy[types.PlatformLocalAgentResp](l.ctx, req, l.svcCtx.CoreCli.DrainLocalAgent)
}
