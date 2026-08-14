package logic

import (
	"context"

	builderpb "appforge/proto/builder"
	"appforge/proto/core"
	"appforge/services/builder/internal/svc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func coreClient(svcCtx *svc.ServiceContext) (core.CoreClient, error) {
	if svcCtx == nil || svcCtx.CoreClient == nil {
		return nil, status.Error(codes.Unavailable, "core rpc client is not configured")
	}
	return svcCtx.CoreClient, nil
}

func toCoreContext(ctx context.Context) context.Context {
	return ctx
}

func mapBuildTask(item *core.BuildTask) *builderpb.BuildTask {
	if item == nil {
		return nil
	}
	return &builderpb.BuildTask{
		Id:                item.Id,
		TenantId:          item.TenantId,
		AppId:             item.AppId,
		VersionId:         item.VersionId,
		ChannelId:         item.ChannelId,
		SigningConfigId:   item.SigningConfigId,
		ChannelCode:       item.ChannelCode,
		VersionCode:       item.VersionCode,
		VersionName:       item.VersionName,
		Status:            builderpb.BuildTaskStatus(item.Status),
		BuilderAttempt:    item.BuilderAttempt,
		Priority:          item.Priority,
		SourceApkUrl:      item.SourceApkUrl,
		BuildConfigJson:   item.BuildConfigJson,
		SourceApkObjectId: item.SourceApkObjectId,
		PoolCode:          item.PoolCode,
		CacheKey:          item.CacheKey,
		CacheEntryId:      item.CacheEntryId,
		CacheHit:          item.CacheHit,
		RetryOfTaskId:     item.RetryOfTaskId,
	}
}

func mapBuilderNode(item *core.BuilderNode) *builderpb.BuilderNode {
	if item == nil {
		return nil
	}
	return &builderpb.BuilderNode{
		Id: item.Id, NodeCode: item.NodeCode, PoolCode: item.PoolCode, Endpoint: item.Endpoint,
		Status: builderpb.BuilderNodeStatus(item.Status), DrainStatus: builderpb.BuilderDrainStatus(item.DrainStatus),
		MaxConcurrency: item.MaxConcurrency, RunningCount: item.RunningCount, DiskFree: item.DiskFree,
		ToolchainVersion: item.ToolchainVersion, BuildProtocolVersion: item.BuildProtocolVersion,
		CapabilityJson: item.CapabilityJson, LastHeartbeatAt: item.LastHeartbeatAt,
	}
}

func mapBuildCacheEntry(item *core.BuildCacheEntry) *builderpb.BuildCacheEntry {
	if item == nil {
		return nil
	}
	return &builderpb.BuildCacheEntry{
		Id: item.Id, TenantId: item.TenantId, CacheKey: item.CacheKey,
		ToolchainVersion: item.ToolchainVersion, BuildProtocolVersion: item.BuildProtocolVersion,
		ArtifactObjectId: item.ArtifactObjectId, ArtifactSha256: item.ArtifactSha256,
		SizeBytes: item.SizeBytes, Status: builderpb.BuildCacheStatus(item.Status), ExpiresAt: item.ExpiresAt,
	}
}

func mapBuildCacheArtifact(item *core.StorageObject) *builderpb.BuildCacheArtifact {
	if item == nil {
		return nil
	}
	return &builderpb.BuildCacheArtifact{
		Id: item.Id, ObjectKey: item.ObjectKey, OriginalName: item.OriginalName,
		ContentType: item.ContentType, SizeBytes: item.SizeBytes, Sha256: item.Sha256,
	}
}

func mapBase(base *core.RespBase) *builderpb.RespBase {
	if base == nil {
		return &builderpb.RespBase{}
	}
	return &builderpb.RespBase{Base: base.Base}
}
