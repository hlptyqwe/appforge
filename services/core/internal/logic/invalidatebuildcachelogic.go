package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type InvalidateBuildCacheLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInvalidateBuildCacheLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InvalidateBuildCacheLogic {
	return &InvalidateBuildCacheLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 管理端失效V4构建缓存条目。
func (l *InvalidateBuildCacheLogic) InvalidateBuildCache(in *core.InvalidateBuildCacheReq) (*core.BuildCacheEntryResp, error) {
	return invalidateBuildCache(l.ctx, l.svcCtx, in)
}
