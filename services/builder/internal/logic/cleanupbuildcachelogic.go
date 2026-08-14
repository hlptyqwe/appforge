package logic

import (
	"context"

	"appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/svc"

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

// 执行V4构建缓存TTL/LRU清理。
func (l *CleanupBuildCacheLogic) CleanupBuildCache(in *builder.CleanupBuildCacheReq) (*builder.CleanupBuildCacheResp, error) {
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.CleanupBuildCache(toCoreContext(l.ctx), &core.CleanupBuildCacheReq{
		Limit: in.GetLimit(), TargetFreeBytes: in.GetTargetFreeBytes(),
	})
	if err != nil {
		return nil, err
	}
	result := &builder.CleanupBuildCacheResult{}
	if resp.Data != nil {
		result.InvalidatedCount = resp.Data.InvalidatedCount
		result.ReclaimableBytes = resp.Data.ReclaimableBytes
		result.ObjectIds = resp.Data.ObjectIds
	}
	return &builder.CleanupBuildCacheResp{Base: resp.Base, Data: result}, nil
}
