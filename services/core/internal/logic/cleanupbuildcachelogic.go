package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CleanupBuildCacheLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCleanupBuildCacheLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CleanupBuildCacheLogic {
	return &CleanupBuildCacheLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 执行V4构建缓存TTL/LRU清理并返回可物理删除对象。
func (l *CleanupBuildCacheLogic) CleanupBuildCache(in *core.CleanupBuildCacheReq) (*core.CleanupBuildCacheResp, error) {
	return cleanupBuildCache(l.ctx, l.svcCtx, in)
}
