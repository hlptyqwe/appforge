package logic

import (
	"appforge/admin-api/internal/types"
	commonpb "appforge/proto/common"
	corepb "appforge/proto/core"
)

func PlatformPage(page types.PageReq) *commonpb.PageReq {
	return &commonpb.PageReq{Cursor: page.Cursor, Limit: page.Limit, Count: page.Count}
}

func PlatformRespBase(base *commonpb.RespBase) types.RespBase {
	if base == nil {
		return types.RespBase{}
	}
	return types.RespBase{Code: base.Code, Msg: base.Msg, Total: base.Total, HasNext: base.HasNext, HasPrev: base.HasPrev, NextCursor: base.NextCursor, PrevCursor: base.PrevCursor}
}

func MapPlatformApplication(item *corepb.Application) types.PlatformApplication {
	if item == nil {
		return types.PlatformApplication{}
	}
	return types.PlatformApplication{Id: item.Id, TenantId: item.TenantId, AppCode: item.AppCode, AppName: item.AppName, PackageName: item.PackageName, Description: item.Description, IconUrl: item.IconUrl, ApiHost: item.ApiHost, Status: int32(item.Status), CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformVersion(item *corepb.Version) types.PlatformVersion {
	if item == nil {
		return types.PlatformVersion{}
	}
	return types.PlatformVersion{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, VersionCode: item.VersionCode, VersionName: item.VersionName, SourceApkUrl: item.SourceApkUrl, SourceApkSha256: item.SourceApkSha256, ReleaseNotes: item.ReleaseNotes, BuildConfigJson: item.BuildConfigJson, Status: int32(item.Status), PublishedAt: item.PublishedAt, CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformChannel(item *corepb.Channel) types.PlatformChannel {
	if item == nil {
		return types.PlatformChannel{}
	}
	return types.PlatformChannel{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, ChannelCode: item.ChannelCode, ChannelName: item.ChannelName, LandingUrl: item.LandingUrl, DownloadUrl: item.DownloadUrl, Status: int32(item.Status), CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformSigningConfig(item *corepb.SigningConfig) types.PlatformSigningConfig {
	if item == nil {
		return types.PlatformSigningConfig{}
	}
	return types.PlatformSigningConfig{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, Name: item.Name, KeystoreObjectKey: item.KeystoreObjectKey, KeyAlias: item.KeyAlias, SecretRef: item.SecretRef, Status: item.Status, LastVerifiedAt: item.LastVerifiedAt, CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformBuildTask(item *corepb.BuildTask) types.PlatformBuildTask {
	if item == nil {
		return types.PlatformBuildTask{}
	}
	return types.PlatformBuildTask{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, VersionId: item.VersionId, ChannelId: item.ChannelId, SigningConfigId: item.SigningConfigId, ChannelCode: item.ChannelCode, VersionCode: item.VersionCode, VersionName: item.VersionName, Status: int32(item.Status), BuilderId: item.BuilderId, BuilderAttempt: item.BuilderAttempt, Priority: item.Priority, ApkUrl: item.ApkUrl, ApkSha256: item.ApkSha256, ApkSize: item.ApkSize, LogUrl: item.LogUrl, ErrorMessage: item.ErrorMessage, QueuedAt: item.QueuedAt, StartTime: item.StartTime, FinishTime: item.FinishTime, CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func MapPlatformStats(item *corepb.ChannelStats) types.PlatformChannelStats {
	if item == nil {
		return types.PlatformChannelStats{}
	}
	return types.PlatformChannelStats{ChannelId: item.ChannelId, ChannelCode: item.ChannelCode, Clicks: item.Clicks, Downloads: item.Downloads, Installs: item.Installs, Registrations: item.Registrations, FirstPays: item.FirstPays, Pays: item.Pays}
}
