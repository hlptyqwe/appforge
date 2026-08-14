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

type ListPlatformBuildCacheEntriesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformBuildCacheEntriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformBuildCacheEntriesLogic {
	return &ListPlatformBuildCacheEntriesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformBuildCacheEntriesLogic) ListPlatformBuildCacheEntries(req *types.ListPlatformBuildCacheEntriesReq) (resp *types.PlatformBuildCacheEntryListResp, err error) {
	item, err := l.svcCtx.CoreCli.ListBuildCacheEntries(l.ctx, &core.BuildCacheEntryListReq{
		Page: platformlogic.PlatformPage(req.PageReq), CacheScope: core.BuildCacheScope(req.CacheScope),
		Status: core.BuildCacheStatus(req.Status), Keyword: req.Keyword,
	})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformBuildCacheEntry, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, platformlogic.MapPlatformBuildCacheEntry(value))
	}
	return &types.PlatformBuildCacheEntryListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}
