package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBuildCacheEntriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBuildCacheEntriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBuildCacheEntriesLogic {
	return &ListBuildCacheEntriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询V4构建缓存条目。
func (l *ListBuildCacheEntriesLogic) ListBuildCacheEntries(in *core.BuildCacheEntryListReq) (*core.BuildCacheEntryListResp, error) {
	return listBuildCacheEntries(l.ctx, l.svcCtx, in)
}
