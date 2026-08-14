package logic

import (
	"encoding/json"

	"appforge/admin-api/internal/types"
	"appforge/common/secretbox"
	"appforge/proto/common"
	"appforge/proto/core"
)

func PlatformPage(page types.PageReq) *common.PageReq {
	return &common.PageReq{Cursor: page.Cursor, Limit: page.Limit, Count: page.Count}
}

func PlatformRespBase(base *common.RespBase) types.RespBase {
	if base == nil {
		return types.RespBase{}
	}
	return types.RespBase{Code: base.Code, Msg: base.Msg, Total: base.Total, HasNext: base.HasNext, HasPrev: base.HasPrev, NextCursor: base.NextCursor, PrevCursor: base.PrevCursor}
}

func MapPlatformApplication(item *core.Application) types.PlatformApplication {
	if item == nil {
		return types.PlatformApplication{}
	}
	return types.PlatformApplication{Id: item.Id, TenantId: item.TenantId, AppCode: item.AppCode, AppName: item.AppName, PackageName: item.PackageName, Description: item.Description, IconUrl: item.IconUrl, ApiHost: item.ApiHost, Status: int32(item.Status), CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformVersion(item *core.Version) types.PlatformVersion {
	if item == nil {
		return types.PlatformVersion{}
	}
	return types.PlatformVersion{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, VersionCode: item.VersionCode, VersionName: item.VersionName, SourceApkUrl: item.SourceApkUrl, SourceApkSha256: item.SourceApkSha256, SourceApkObjectId: item.SourceApkObjectId, ReleaseNotes: item.ReleaseNotes, BuildConfigJson: item.BuildConfigJson, Status: int32(item.Status), PublishedAt: item.PublishedAt, CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformChannel(item *core.Channel) types.PlatformChannel {
	if item == nil {
		return types.PlatformChannel{}
	}
	return types.PlatformChannel{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, ChannelCode: item.ChannelCode, ChannelName: item.ChannelName, LandingUrl: item.LandingUrl, DownloadUrl: item.DownloadUrl, Status: int32(item.Status), CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformSigningConfig(item *core.SigningConfig) types.PlatformSigningConfig {
	if item == nil {
		return types.PlatformSigningConfig{}
	}
	return types.PlatformSigningConfig{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, Name: item.Name, KeystoreObjectKey: item.KeystoreObjectKey, KeystoreObjectId: item.KeystoreObjectId, KeyAlias: item.KeyAlias, SecretRef: item.SecretRef, Status: item.Status, LastVerifiedAt: item.LastVerifiedAt, CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime, CertificateSha256: item.CertificateSha256}
}

func MapPlatformBuildTask(item *core.BuildTask) types.PlatformBuildTask {
	if item == nil {
		return types.PlatformBuildTask{}
	}
	return types.PlatformBuildTask{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, VersionId: item.VersionId, ChannelId: item.ChannelId, SigningConfigId: item.SigningConfigId, ChannelCode: item.ChannelCode, VersionCode: item.VersionCode, VersionName: item.VersionName, Status: int32(item.Status), BuilderId: item.BuilderId, BuilderAttempt: item.BuilderAttempt, Priority: item.Priority, ApkUrl: item.ApkUrl, ApkSha256: item.ApkSha256, ApkSize: item.ApkSize, LogUrl: item.LogUrl, SourceApkObjectId: item.SourceApkObjectId, ApkObjectId: item.ApkObjectId, LogObjectId: item.LogObjectId, ErrorMessage: item.ErrorMessage, QueuedAt: item.QueuedAt, StartTime: item.StartTime, FinishTime: item.FinishTime, CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime, BrandingProfileId: item.BrandingProfileId, BrandingRevision: item.BrandingRevision, BrandingSnapshotJson: item.BrandingSnapshotJson, WhiteLabelProductId: item.WhiteLabelProductId, TemplateRevision: item.TemplateRevision, TemplateSnapshotJson: maskTemplateSnapshot(item.TemplateSnapshotJson), PoolCode: item.PoolCode, CacheKey: item.CacheKey, CacheEntryId: item.CacheEntryId, CacheHit: item.CacheHit, CancelRequestedAt: item.CancelRequestedAt, CancelledAt: item.CancelledAt, CancelReason: item.CancelReason, RetryOfTaskId: item.RetryOfTaskId}
}

func MapPlatformBuilderNode(item *core.BuilderNode) types.PlatformBuilderNode {
	if item == nil {
		return types.PlatformBuilderNode{}
	}
	return types.PlatformBuilderNode{Id: item.Id, NodeCode: item.NodeCode, PoolCode: item.PoolCode,
		Endpoint: item.Endpoint, Status: int32(item.Status), DrainStatus: int32(item.DrainStatus),
		MaxConcurrency: item.MaxConcurrency, RunningCount: item.RunningCount, CpuCapacity: item.CpuCapacity,
		MemoryCapacity: item.MemoryCapacity, DiskCapacity: item.DiskCapacity, DiskFree: item.DiskFree,
		ToolchainVersion: item.ToolchainVersion, BuildProtocolVersion: item.BuildProtocolVersion,
		CapabilityJson: item.CapabilityJson, LastErrorMessage: item.LastErrorMessage,
		LastHeartbeatAt: item.LastHeartbeatAt, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformBuildConcurrencyPolicy(item *core.BuildConcurrencyPolicy) types.PlatformBuildConcurrencyPolicy {
	if item == nil {
		return types.PlatformBuildConcurrencyPolicy{}
	}
	return types.PlatformBuildConcurrencyPolicy{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId,
		PoolCode: item.PoolCode, MaxConcurrency: item.MaxConcurrency, FairWeight: item.FairWeight,
		MaxPriority: item.MaxPriority, Status: int32(item.Status), CreateBy: item.CreateBy,
		CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformBuildCacheEntry(item *core.BuildCacheEntry) types.PlatformBuildCacheEntry {
	if item == nil {
		return types.PlatformBuildCacheEntry{}
	}
	return types.PlatformBuildCacheEntry{Id: item.Id, TenantId: item.TenantId, CacheScope: int32(item.CacheScope),
		CacheKey: item.CacheKey, ToolchainVersion: item.ToolchainVersion,
		BuildProtocolVersion: item.BuildProtocolVersion, InputDigest: item.InputDigest,
		ArtifactObjectId: item.ArtifactObjectId, ArtifactSha256: item.ArtifactSha256,
		SizeBytes: item.SizeBytes, HitCount: item.HitCount, Status: int32(item.Status),
		ExpiresAt: item.ExpiresAt, LastHitAt: item.LastHitAt, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformBuildSchedulerEvent(item *core.BuildSchedulerEvent) types.PlatformBuildSchedulerEvent {
	if item == nil {
		return types.PlatformBuildSchedulerEvent{}
	}
	return types.PlatformBuildSchedulerEvent{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId,
		TaskId: item.TaskId, NodeCode: item.NodeCode, PoolCode: item.PoolCode, EventType: int32(item.EventType),
		ReasonCode: item.ReasonCode, DecisionJson: item.DecisionJson, CreateTime: item.CreateTime}
}

func MapPlatformOpenApiCredential(item *core.OpenApiCredential) types.PlatformOpenApiCredential {
	if item == nil {
		return types.PlatformOpenApiCredential{}
	}
	scopes := make([]int32, 0, len(item.Scopes))
	for _, scope := range item.Scopes {
		scopes = append(scopes, int32(scope))
	}
	return types.PlatformOpenApiCredential{
		Id: item.Id, TenantId: item.TenantId, CredentialName: item.CredentialName, KeyId: item.KeyId,
		Scopes: scopes, AppIds: item.AppIds, IpAllowlist: item.IpAllowlist,
		RateLimitPerMinute: item.RateLimitPerMinute, Status: int32(item.Status),
		ExpiresAt: item.ExpiresAt, GraceExpiresAt: item.GraceExpiresAt,
		RotatedFromId: item.RotatedFromId, LastUsedAt: item.LastUsedAt,
		CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime,
	}
}

func MapPlatformWhiteLabelTemplate(item *core.WhiteLabelTemplate) types.PlatformWhiteLabelTemplate {
	if item == nil {
		return types.PlatformWhiteLabelTemplate{}
	}
	return types.PlatformWhiteLabelTemplate{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId,
		TemplateCode: item.TemplateCode, TemplateName: item.TemplateName, SourceVersionId: item.SourceVersionId,
		ParameterSchemaJson: item.ParameterSchemaJson, CompatibilityRulesJson: item.CompatibilityRulesJson,
		Status: int32(item.Status), PublishedRevision: item.PublishedRevision,
		CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformWhiteLabelTemplateRevision(item *core.WhiteLabelTemplateRevision) types.PlatformWhiteLabelTemplateRevision {
	if item == nil {
		return types.PlatformWhiteLabelTemplateRevision{}
	}
	return types.PlatformWhiteLabelTemplateRevision{Id: item.Id, TenantId: item.TenantId, TemplateId: item.TemplateId,
		Revision: item.Revision, PackageNameRuleJson: item.PackageNameRuleJson, ManifestPatchJson: item.ManifestPatchJson,
		ResourcePatchJson: item.ResourcePatchJson, ExtensionFilesJson: item.ExtensionFilesJson,
		ExpectedArtifactsJson: item.ExpectedArtifactsJson, Checksum: item.Checksum,
		Status: int32(item.Status), CreateTime: item.CreateTime}
}

func MapPlatformWhiteLabelProduct(item *core.WhiteLabelProduct) types.PlatformWhiteLabelProduct {
	if item == nil {
		return types.PlatformWhiteLabelProduct{}
	}
	return types.PlatformWhiteLabelProduct{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId,
		ProductCode: item.ProductCode, ProductName: item.ProductName, TemplateId: item.TemplateId,
		TemplateRevision: item.TemplateRevision, BrandingProfileId: item.BrandingProfileId,
		PackageName: item.PackageName, SigningConfigId: item.SigningConfigId,
		ParameterValuesJson: maskSealedJSON(item.ParameterValuesJson), Status: int32(item.Status),
		CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func maskSealedJSON(raw string) string {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return `{}`
	}
	value = maskSealedValue(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		return `{}`
	}
	return string(encoded)
}

func maskTemplateSnapshot(raw string) string {
	if raw == "" {
		return ""
	}
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return `{}`
	}
	if parameters, ok := snapshot["parameterValuesJson"].(string); ok {
		snapshot["parameterValuesJson"] = maskSealedJSON(parameters)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return `{}`
	}
	return string(encoded)
}

func maskSealedValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = maskSealedValue(child)
		}
		return typed
	case []any:
		for index, child := range typed {
			typed[index] = maskSealedValue(child)
		}
		return typed
	case string:
		if secretbox.IsSealed(typed) {
			return "***"
		}
	}
	return value
}

func MapPlatformBrandingProfile(item *core.BrandingProfile) types.PlatformBrandingProfile {
	if item == nil {
		return types.PlatformBrandingProfile{}
	}
	return types.PlatformBrandingProfile{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId,
		ProfileName: item.ProfileName, AppName: item.AppName, LogoObjectId: item.LogoObjectId,
		SplashObjectId: item.SplashObjectId, ApiHost: item.ApiHost, RewriteMode: int32(item.RewriteMode),
		LauncherIconTarget: item.LauncherIconTarget, SplashResourceTarget: item.SplashResourceTarget,
		RuntimeConfigJson: item.RuntimeConfigJson, Status: int32(item.Status), Revision: item.Revision,
		CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformBrandingPreflight(item *core.BrandingPreflight) types.PlatformBrandingPreflight {
	if item == nil {
		return types.PlatformBrandingPreflight{}
	}
	return types.PlatformBrandingPreflight{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId,
		BrandingProfileId: item.BrandingProfileId, BrandingRevision: item.BrandingRevision,
		VersionId: item.VersionId, Status: int32(item.Status), ReportJson: item.ReportJson,
		SourceApkSha256: item.SourceApkSha256, ToolchainVersion: item.ToolchainVersion,
		StartTime: item.StartTime, FinishTime: item.FinishTime, CreateBy: item.CreateBy,
		CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformStats(item *core.ChannelStats) types.PlatformChannelStats {
	if item == nil {
		return types.PlatformChannelStats{}
	}
	return types.PlatformChannelStats{ChannelId: item.ChannelId, ChannelCode: item.ChannelCode, Clicks: item.Clicks, Downloads: item.Downloads, Installs: item.Installs, Registrations: item.Registrations, FirstPays: item.FirstPays, Pays: item.Pays}
}
