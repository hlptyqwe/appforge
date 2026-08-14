package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveBuildCacheLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveBuildCacheLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveBuildCacheLogic {
	return &ResolveBuildCacheLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 解析V4输入可寻址构建缓存，并在Worker字节校验后确认命中。
func (l *ResolveBuildCacheLogic) ResolveBuildCache(in *core.ResolveBuildCacheReq) (*core.BuildCacheResolutionResp, error) {
	return resolveBuildCache(l.ctx, l.svcCtx, in)
}
