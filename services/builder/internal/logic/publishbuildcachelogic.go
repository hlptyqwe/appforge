package logic

import (
	"context"

	"appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/svc"

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

// 发布V4独立构建缓存产物。
func (l *PublishBuildCacheLogic) PublishBuildCache(in *builder.PublishBuildCacheReq) (*builder.BuildCacheEntryResp, error) {
	client, err := coreClient(l.svcCtx)
	if err != nil {
		return nil, err
	}
	resp, err := client.PublishBuildCache(toCoreContext(l.ctx), &core.PublishBuildCacheReq{
		TaskId: in.GetTaskId(), NodeCode: in.GetNodeCode(), BuilderAttempt: in.GetBuilderAttempt(),
		ToolchainVersion: in.GetToolchainVersion(), BuildProtocolVersion: in.GetBuildProtocolVersion(),
		ArtifactObjectId: in.GetArtifactObjectId(), ArtifactSha256: in.GetArtifactSha256(),
		SizeBytes: in.GetSizeBytes(), TtlSeconds: in.GetTtlSeconds(), ArtifactObjectKey: in.GetArtifactObjectKey(),
	})
	if err != nil {
		return nil, err
	}
	return &builder.BuildCacheEntryResp{Base: resp.Base, Data: mapBuildCacheEntry(resp.Data)}, nil
}
