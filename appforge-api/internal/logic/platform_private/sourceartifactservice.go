package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/sourceoauth"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func importPlatformSourceArtifact(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ImportPlatformSourceArtifactReq) (*types.PlatformSourceArtifactImportResp, error) {
	if req == nil || req.AppId <= 0 || req.RepositoryId <= 0 || req.VersionCode <= 0 || req.VersionName == "" {
		return nil, status.Error(codes.InvalidArgument, "appId, repositoryId, versionCode and versionName are required")
	}
	imported, err := sourceoauth.ImportArtifact(ctx, svcCtx, req.AppId, req.RepositoryId, core.SourceArtifactType(req.ArtifactSource),
		req.ExternalArtifactId, req.ReleaseRef, req.VersionCode, req.VersionName, req.ReleaseNotes)
	if err != nil {
		return nil, err
	}
	return &types.PlatformSourceArtifactImportResp{RespBase: platformlogic.PlatformRespBase(imported.Base), Data: types.PlatformSourceArtifactImportResult{
		Version: platformlogic.MapPlatformVersion(imported.Data.Version), Artifact: mapPlatformSourceArtifact(imported.Data.Artifact),
	}}, nil
}

func mapPlatformSourceArtifact(item *core.SourceArtifact) types.PlatformSourceArtifact {
	if item == nil {
		return types.PlatformSourceArtifact{}
	}
	return types.PlatformSourceArtifact{Id: item.Id, AppId: item.AppId, VersionId: item.VersionId, IntegrationId: item.IntegrationId,
		RepositoryId: item.RepositoryId, ArtifactSource: int32(item.ArtifactSource), ExternalArtifactId: item.ExternalArtifactId,
		CommitSha: item.CommitSha, PipelineRef: item.PipelineRef, JobRef: item.JobRef, ArtifactSha256: item.ArtifactSha256,
		StorageObjectId: item.StorageObjectId, CreateTime: item.CreateTime}
}
