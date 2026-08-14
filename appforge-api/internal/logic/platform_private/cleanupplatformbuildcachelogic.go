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

type CleanupPlatformBuildCacheLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCleanupPlatformBuildCacheLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CleanupPlatformBuildCacheLogic {
	return &CleanupPlatformBuildCacheLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CleanupPlatformBuildCacheLogic) CleanupPlatformBuildCache(req *types.CleanupPlatformBuildCacheReq) (resp *types.PlatformBuildCacheCleanupResp, err error) {
	item, err := l.svcCtx.CoreCli.CleanupBuildCache(l.ctx, &core.CleanupBuildCacheReq{
		Limit: req.Limit, TargetFreeBytes: req.TargetFreeBytes,
	})
	if err != nil {
		return nil, err
	}
	result := types.PlatformBuildCacheCleanupResult{}
	if item.Data != nil {
		result.InvalidatedCount = int64(item.Data.InvalidatedCount)
		result.ReclaimableBytes = item.Data.ReclaimableBytes
		result.ObjectIds = item.Data.ObjectIds
	}
	return &types.PlatformBuildCacheCleanupResp{
		RespBase: platformlogic.PlatformRespBase(item.Base), Data: result,
	}, nil
}
