package platform_private

import (
	"context"
	"net/url"
	"strings"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/common"
	"appforge/proto/core"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapPlatformSourceBuildTrigger(item *core.SourceBuildTrigger) types.PlatformSourceBuildTrigger {
	if item == nil {
		return types.PlatformSourceBuildTrigger{}
	}
	return types.PlatformSourceBuildTrigger{Id: item.Id, TenantId: item.TenantId, RepositoryId: item.RepositoryId,
		AppId: item.AppId, TriggerName: item.TriggerName, EventType: int32(item.EventType), RefPattern: item.RefPattern,
		ArtifactSelector: item.ArtifactSelector, ChannelIds: item.ChannelIds, SigningConfigId: item.SigningConfigId,
		BrandingProfileId: item.BrandingProfileId, WhiteLabelProductId: item.WhiteLabelProductId, Priority: item.Priority,
		PoolCode: item.PoolCode, VersionNamePrefix: item.VersionNamePrefix, Status: int32(item.Status),
		Platform: int32(item.Platform), RepositoryFullName: item.RepositoryFullName, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func mapPlatformSourceWebhookEvent(item *core.SourceWebhookEvent) types.PlatformSourceWebhookEvent {
	if item == nil {
		return types.PlatformSourceWebhookEvent{}
	}
	return types.PlatformSourceWebhookEvent{Id: item.Id, TriggerId: item.TriggerId, ProviderEventId: item.ProviderEventId,
		ProviderEventType: item.ProviderEventType, SourceRef: item.SourceRef, CommitSha: item.CommitSha,
		ArtifactSource: int32(item.ArtifactSource), ExternalArtifactId: item.ExternalArtifactId, ReleaseRef: item.ReleaseRef,
		PipelineRef: item.PipelineRef, JobRef: item.JobRef, PayloadSha256: item.PayloadSha256, VersionCode: item.VersionCode,
		VersionName: item.VersionName, Status: int32(item.Status), Attempt: item.Attempt, VersionId: item.VersionId,
		BuildTaskIds: item.BuildTaskIds, ErrorMessage: item.ErrorMessage, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func sourceWebhookURL(svcCtx *svc.ServiceContext, platform core.SourcePlatform, token string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(svcCtx.Config.SourceOAuth.WebhookBaseURL))
	if err != nil || base.Scheme == "" || base.Host == "" || (base.Scheme != "https" && base.Hostname() != "localhost") {
		return "", status.Error(codes.FailedPrecondition, "source webhook base URL is not configured with HTTPS")
	}
	provider := ""
	switch platform {
	case core.SourcePlatform_SOURCE_PLATFORM_GITHUB:
		provider = "github"
	case core.SourcePlatform_SOURCE_PLATFORM_GITLAB:
		provider = "gitlab"
	default:
		return "", status.Error(codes.Internal, "source trigger provider is invalid")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + provider + "/" + url.PathEscape(token)
	base.RawQuery, base.Fragment = "", ""
	return base.String(), nil
}

func mapSourceBuildTriggerSecret(svcCtx *svc.ServiceContext, response *core.SourceBuildTriggerSecretResp) (*types.PlatformSourceBuildTriggerSecretResp, error) {
	if response == nil || response.Data == nil || response.Data.Trigger == nil {
		return nil, status.Error(codes.Internal, "source build trigger secret response is empty")
	}
	webhookURL, err := sourceWebhookURL(svcCtx, response.Data.Trigger.Platform, response.Data.WebhookToken)
	if err != nil {
		return nil, err
	}
	return &types.PlatformSourceBuildTriggerSecretResp{RespBase: platformlogic.PlatformRespBase(response.Base),
		Data: types.PlatformSourceBuildTriggerSecret{Trigger: mapPlatformSourceBuildTrigger(response.Data.Trigger),
			WebhookUrl: webhookURL, SigningSecret: response.Data.WebhookSecret}}, nil
}

func createPlatformSourceBuildTrigger(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformSourceBuildTriggerReq) (*types.PlatformSourceBuildTriggerSecretResp, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	response, err := svcCtx.CoreCli.CreateSourceBuildTrigger(ctx, &core.CreateSourceBuildTriggerReq{RepositoryId: req.RepositoryId,
		AppId: req.AppId, TriggerName: req.TriggerName, EventType: core.SourceBuildTriggerEventType(req.EventType),
		RefPattern: req.RefPattern, ArtifactSelector: req.ArtifactSelector, ChannelIds: req.ChannelIds,
		SigningConfigId: req.SigningConfigId, BrandingProfileId: req.BrandingProfileId,
		WhiteLabelProductId: req.WhiteLabelProductId, Priority: req.Priority, PoolCode: req.PoolCode,
		VersionNamePrefix: req.VersionNamePrefix})
	if err != nil {
		return nil, err
	}
	return mapSourceBuildTriggerSecret(svcCtx, response)
}

func updatePlatformSourceBuildTrigger(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UpdatePlatformSourceBuildTriggerReq) (*types.PlatformSourceBuildTriggerResp, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	response, err := svcCtx.CoreCli.UpdateSourceBuildTrigger(ctx, &core.UpdateSourceBuildTriggerReq{Id: req.Id,
		TriggerName: req.TriggerName, EventType: core.SourceBuildTriggerEventType(req.EventType), RefPattern: req.RefPattern,
		ArtifactSelector: req.ArtifactSelector, ChannelIds: req.ChannelIds, SigningConfigId: req.SigningConfigId,
		BrandingProfileId: req.BrandingProfileId, WhiteLabelProductId: req.WhiteLabelProductId,
		Priority: req.Priority, PoolCode: req.PoolCode, VersionNamePrefix: req.VersionNamePrefix,
		Status: core.SourceBuildTriggerStatus(req.Status)})
	if err != nil {
		return nil, err
	}
	return &types.PlatformSourceBuildTriggerResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: mapPlatformSourceBuildTrigger(response.Data)}, nil
}

func getPlatformSourceBuildTrigger(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.PlatformSourceBuildTriggerResp, error) {
	response, err := svcCtx.CoreCli.GetSourceBuildTrigger(ctx, &core.SourceBuildTriggerIdReq{Id: id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformSourceBuildTriggerResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: mapPlatformSourceBuildTrigger(response.Data)}, nil
}

func listPlatformSourceBuildTriggers(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformSourceBuildTriggersReq) (*types.PlatformSourceBuildTriggerListResp, error) {
	if req == nil {
		req = &types.ListPlatformSourceBuildTriggersReq{}
	}
	response, err := svcCtx.CoreCli.ListSourceBuildTriggers(ctx, &core.SourceBuildTriggerListReq{Page: &common.PageReq{
		Limit: req.Limit, Cursor: req.Cursor, Count: req.Count}, RepositoryId: req.RepositoryId, AppId: req.AppId,
		Status: core.SourceBuildTriggerStatus(req.Status), Keyword: req.Keyword})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformSourceBuildTrigger, 0, len(response.Data))
	for _, item := range response.Data {
		data = append(data, mapPlatformSourceBuildTrigger(item))
	}
	return &types.PlatformSourceBuildTriggerListResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: data}, nil
}

func rotatePlatformSourceBuildTriggerSecret(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.PlatformSourceBuildTriggerSecretResp, error) {
	response, err := svcCtx.CoreCli.RotateSourceBuildTriggerSecret(ctx, &core.SourceBuildTriggerIdReq{Id: id})
	if err != nil {
		return nil, err
	}
	return mapSourceBuildTriggerSecret(svcCtx, response)
}

func listPlatformSourceWebhookEvents(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformSourceWebhookEventsReq) (*types.PlatformSourceWebhookEventListResp, error) {
	if req == nil {
		req = &types.ListPlatformSourceWebhookEventsReq{}
	}
	response, err := svcCtx.CoreCli.ListSourceWebhookEvents(ctx, &core.SourceWebhookEventListReq{Page: &common.PageReq{
		Limit: req.Limit, Cursor: req.Cursor, Count: req.Count}, TriggerId: req.TriggerId, Status: core.SourceWebhookEventStatus(req.Status)})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformSourceWebhookEvent, 0, len(response.Data))
	for _, item := range response.Data {
		data = append(data, mapPlatformSourceWebhookEvent(item))
	}
	return &types.PlatformSourceWebhookEventListResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: data}, nil
}
