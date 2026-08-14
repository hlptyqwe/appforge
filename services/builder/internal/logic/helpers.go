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
	}
}

func mapBase(base *core.RespBase) *builderpb.RespBase {
	if base == nil {
		return &builderpb.RespBase{}
	}
	return &builderpb.RespBase{Base: base.Base}
}
