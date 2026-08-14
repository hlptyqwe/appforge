// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformSourceBuildTriggersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformSourceBuildTriggersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformSourceBuildTriggersLogic {
	return &ListPlatformSourceBuildTriggersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformSourceBuildTriggersLogic) ListPlatformSourceBuildTriggers(req *types.ListPlatformSourceBuildTriggersReq) (resp *types.PlatformSourceBuildTriggerListResp, err error) {
	return listPlatformSourceBuildTriggers(l.ctx, l.svcCtx, req)
}
