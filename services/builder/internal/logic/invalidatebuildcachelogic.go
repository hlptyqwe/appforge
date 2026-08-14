package logic

import (
	"context"

	"appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/svc"

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

// 失效损坏或不再可信的V4构建缓存。
func (l *InvalidateBuildCacheLogic) InvalidateBuildCache(in *builder.InvalidateBuildCacheReq) (*builder.BuildCacheEntryResp, error) {
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.InvalidateBuildCache(toCoreContext(l.ctx), &core.InvalidateBuildCacheReq{
		Id: in.GetId(), TaskId: in.GetTaskId(), NodeCode: in.GetNodeCode(), Reason: in.GetReason(),
	})
	if err != nil {
		return nil, err
	}
	return &builder.BuildCacheEntryResp{Base: resp.Base, Data: mapBuildCacheEntry(resp.Data)}, nil
}
