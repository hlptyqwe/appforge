package logic

import (
	"context"

	"appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/svc"

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

// 解析V4输入可寻址构建缓存。
func (l *ResolveBuildCacheLogic) ResolveBuildCache(in *builder.ResolveBuildCacheReq) (*builder.BuildCacheResolutionResp, error) {
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.ResolveBuildCache(toCoreContext(l.ctx), &core.ResolveBuildCacheReq{
		TaskId: in.GetTaskId(), NodeCode: in.GetNodeCode(), BuilderAttempt: in.GetBuilderAttempt(),
		ToolchainVersion: in.GetToolchainVersion(), BuildProtocolVersion: in.GetBuildProtocolVersion(),
		ConfirmHit: in.GetConfirmHit(),
	})
	if err != nil {
		return nil, err
	}
	result := &builder.BuildCacheResolution{}
	if resp.Data != nil {
		result.Hit = resp.Data.Hit
		result.Entry = mapBuildCacheEntry(resp.Data.Entry)
		result.Artifact = mapBuildCacheArtifact(resp.Data.Artifact)
	}
	return &builder.BuildCacheResolutionResp{Base: resp.Base, Data: result}, nil
}
