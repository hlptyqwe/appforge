package sourceoauth

import (
	"context"
	"os"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/proto/core"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ImportArtifact downloads one provider-controlled APK, verifies it, stores it privately,
// and atomically creates the version plus immutable provenance in Core.
func ImportArtifact(ctx context.Context, svcCtx *svc.ServiceContext, appID, repositoryID int64,
	artifactSource core.SourceArtifactType, externalArtifactID, releaseRef string, versionCode int64,
	versionName, releaseNotes string) (*core.SourceArtifactImportResp, error) {
	fetched, err := FetchArtifact(ctx, svcCtx, repositoryID, artifactSource, externalArtifactID, releaseRef)
	if err != nil {
		return nil, err
	}
	defer os.Remove(fetched.FilePath)
	store, err := platformlogic.LoadObjectStore(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	objectKey, err := platformlogic.GenerateStorageObjectKey(ctx, core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK, fetched.FileName)
	if err != nil {
		return nil, err
	}
	createdObject, err := svcCtx.CoreCli.CreateStorageObject(ctx, &core.CreateStorageObjectReq{AppId: appID,
		ObjectType: core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK, ObjectKey: objectKey, OriginalName: fetched.FileName,
		ContentType: "application/vnd.android.package-archive", SizeBytes: fetched.Size})
	if err != nil {
		return nil, err
	}
	file, err := os.Open(fetched.FilePath)
	if err != nil {
		_, _ = svcCtx.CoreCli.FailStorageObject(ctx, &core.FailStorageObjectReq{Id: createdObject.Data.Id})
		return nil, status.Error(codes.Internal, "open imported APK failed")
	}
	putErr := store.PutObject(ctx, objectKey, file, fetched.Size, "application/vnd.android.package-archive")
	_ = file.Close()
	if putErr != nil {
		platformlogic.CleanupFailedUpload(ctx, svcCtx, store, createdObject.Data)
		return nil, status.Errorf(codes.Internal, "store imported APK failed: %v", putErr)
	}
	size, verifiedSHA, err := platformlogic.VerifyStorageObject(ctx, store, createdObject.Data)
	if err != nil {
		platformlogic.CleanupFailedUpload(ctx, svcCtx, store, createdObject.Data)
		return nil, err
	}
	if verifiedSHA != fetched.SHA256 {
		platformlogic.CleanupFailedUpload(ctx, svcCtx, store, createdObject.Data)
		return nil, status.Error(codes.FailedPrecondition, "provider APK changed during import")
	}
	completedObject, err := svcCtx.CoreCli.CompleteStorageObject(ctx, &core.CompleteStorageObjectReq{Id: createdObject.Data.Id, SizeBytes: size, Sha256: verifiedSHA})
	if err != nil {
		platformlogic.CleanupFailedUpload(ctx, svcCtx, store, createdObject.Data)
		return nil, err
	}
	imported, err := svcCtx.CoreCli.ImportSourceArtifact(ctx, &core.ImportSourceArtifactReq{AppId: appID,
		VersionCode: versionCode, VersionName: versionName, ReleaseNotes: releaseNotes,
		IntegrationId: fetched.IntegrationID, RepositoryId: fetched.RepositoryID, ArtifactSource: fetched.ArtifactSource,
		ExternalArtifactId: fetched.ExternalArtifactID, CommitSha: fetched.CommitSHA, PipelineRef: fetched.PipelineRef,
		JobRef: fetched.JobRef, ArtifactSha256: verifiedSHA, StorageObjectId: completedObject.Data.Id})
	if err != nil {
		platformlogic.CleanupFailedUpload(ctx, svcCtx, store, completedObject.Data)
		return nil, err
	}
	if imported.GetData().GetArtifact() == nil {
		platformlogic.CleanupFailedUpload(ctx, svcCtx, store, completedObject.Data)
		return nil, status.Error(codes.Internal, "source artifact import response is empty")
	}
	if imported.Data.Artifact.StorageObjectId != completedObject.Data.Id {
		// An earlier Worker attempt already committed the same immutable provider Artifact.
		platformlogic.CleanupFailedUpload(ctx, svcCtx, store, completedObject.Data)
	}
	return imported, nil
}
