// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type InvalidatePlatformBuildCacheLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewInvalidatePlatformBuildCacheLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InvalidatePlatformBuildCacheLogic {
	return &InvalidatePlatformBuildCacheLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *InvalidatePlatformBuildCacheLogic) InvalidatePlatformBuildCache(req *types.InvalidatePlatformBuildCacheReq) (resp *types.PlatformBuildCacheEntryResp, err error) {
	item, err := l.svcCtx.CoreCli.InvalidateBuildCache(l.ctx, &core.InvalidateBuildCacheReq{Id: req.Id, Reason: req.Reason})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBuildCacheEntryResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBuildCacheEntry(item.Data)}, nil
}
