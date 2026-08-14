package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishBuildCacheLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishBuildCacheLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishBuildCacheLogic {
	return &PublishBuildCacheLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 发布独立V4构建缓存产物。
func (l *PublishBuildCacheLogic) PublishBuildCache(in *core.PublishBuildCacheReq) (*core.BuildCacheEntryResp, error) {
	return publishBuildCache(l.ctx, l.svcCtx, in)
}
