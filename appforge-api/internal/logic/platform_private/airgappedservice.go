package platform_private

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/common/airgap"
	"appforge/common/storage"
	"appforge/proto/core"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type airGappedBuildManifest struct {
	SchemaVersion           int32                 `json:"schema_version"`
	Task                    *airGappedBuildTask   `json:"task"`
	PackageName             string                `json:"package_name"`
	APIHost                 string                `json:"api_host"`
	ChannelName             string                `json:"channel_name"`
	LandingURL              string                `json:"landing_url"`
	KeyAlias                string                `json:"key_alias"`
	SigningSecretRef        string                `json:"signing_secret_ref"`
	SignerCertificateSHA256 string                `json:"signer_certificate_sha256"`
	BrandingSnapshotJSON    string                `json:"branding_snapshot_json,omitempty"`
	TemplateSnapshotJSON    string                `json:"template_snapshot_json,omitempty"`
	Inputs                  []airGappedBuildInput `json:"inputs"`
}

type airGappedBuildTask struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenant_id"`
	AppID          int64  `json:"app_id"`
	VersionID      int64  `json:"version_id"`
	BuilderAttempt int32  `json:"builder_attempt"`
	ChannelCode    string `json:"channel_code"`
	VersionCode    int64  `json:"version_code"`
	VersionName    string `json:"version_name"`
}

type airGappedBuildInput struct {
	Role         string                 `json:"role"`
	ObjectID     int64                  `json:"object_id"`
	ObjectType   core.StorageObjectType `json:"object_type"`
	OriginalName string                 `json:"original_name"`
	ContentType  string                 `json:"content_type"`
	SizeBytes    int64                  `json:"size_bytes"`
	SHA256       string                 `json:"sha256"`
}

func prepareAirGappedExport(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PreparePlatformAirGappedExportReq) (*types.PlatformAirGappedExportResp, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	prepared, err := svcCtx.CoreCli.PrepareAirGappedExport(ctx, &core.PrepareAirGappedExportReq{
		AgentId: req.AgentId, TaskId: req.TaskId, ExpiresSeconds: req.ExpiresSeconds,
	})
	if err != nil {
		return nil, err
	}
	if prepared.GetPackage() == nil || prepared.GetExecution() == nil || prepared.GetNonce() == "" {
		return nil, status.Error(codes.FailedPrecondition, "AIR_GAPPED preparation response is incomplete")
	}
	completed := false
	defer func() {
		if !completed {
			abortCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			_, _ = svcCtx.CoreCli.AbortAirGappedExport(abortCtx, &core.AbortAirGappedExportReq{
				PackageCode: prepared.Package.PackageCode, Reason: "AIR_GAPPED_EXPORT_GENERATION_FAILED",
			})
		}
	}()
	bundle, artifacts, objects, err := buildAirGappedTaskBundle(prepared.Execution)
	if err != nil {
		return nil, err
	}
	bundleJSON, err := json.Marshal(bundle)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "encode AIR_GAPPED task bundle: %v", err)
	}
	manifest := airgap.TaskManifest{SchemaVersion: airgap.SchemaVersion, PackageCode: prepared.Package.PackageCode,
		Nonce: prepared.Nonce, TenantID: prepared.Package.TenantId, AgentID: prepared.Package.AgentId,
		AgentCertificateSerial: prepared.Package.AgentCertificateSerial, TaskID: prepared.Package.TaskId,
		BuilderAttempt: prepared.Package.BuilderAttempt, IssuedAt: prepared.Package.IssuedAt,
		ExpiresAt: prepared.Package.ExpiresAt, Bundle: bundleJSON, Inputs: artifacts}
	canonical, err := airgap.CanonicalJSON(manifest)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "encode AIR_GAPPED manifest: %v", err)
	}
	signed, err := svcCtx.CoreCli.SignAirGappedManifest(ctx, &core.SignAirGappedManifestReq{
		PackageCode: prepared.Package.PackageCode, ManifestJson: string(canonical),
	})
	if err != nil {
		return nil, err
	}
	envelope := airgap.TaskEnvelope{Manifest: manifest, Signature: airgap.Signature{Algorithm: signed.Algorithm, Value: signed.SignatureBase64}}
	store, err := platformlogic.LoadObjectStore(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	packageFile, err := os.CreateTemp("", "appforge-air-gapped-task-*.zip")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create AIR_GAPPED task file: %v", err)
	}
	packagePath := packageFile.Name()
	defer os.Remove(packagePath)
	defer packageFile.Close()
	if err := airgap.WriteTaskPackage(packageFile, envelope, func(artifact airgap.Artifact) (io.ReadCloser, error) {
		object := objects[artifact.ObjectID]
		if object == nil || object.ObjectKey == "" {
			return nil, status.Error(codes.FailedPrecondition, "AIR_GAPPED input object mapping is unavailable")
		}
		return store.OpenObject(ctx, object.ObjectKey)
	}); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "write AIR_GAPPED task package: %v", err)
	}
	if err := packageFile.Sync(); err != nil {
		return nil, status.Errorf(codes.Internal, "sync AIR_GAPPED task package: %v", err)
	}
	object, err := putVerifiedFile(ctx, svcCtx, store, prepared.Package.AppId,
		core.StorageObjectType_STORAGE_OBJECT_TYPE_OFFLINE_TASK_PACKAGE, prepared.Package.PackageCode+".zip",
		"application/zip", packageFile)
	if err != nil {
		return nil, err
	}
	finalized, err := svcCtx.CoreCli.FinalizeAirGappedExport(ctx, &core.FinalizeAirGappedExportReq{
		PackageCode: prepared.Package.PackageCode, ExportObjectId: object.Id, ExportSha256: object.Sha256, ExportSizeBytes: object.SizeBytes,
	})
	if err != nil {
		platformlogic.CleanupFailedUpload(ctx, svcCtx, store, object)
		return nil, err
	}
	downloadTTL := 5 * time.Minute
	downloadURL, err := store.PresignGet(ctx, object.ObjectKey, downloadTTL)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create AIR_GAPPED download URL: %v", err)
	}
	completed = true
	return &types.PlatformAirGappedExportResp{RespBase: platformlogic.PlatformRespBase(finalized.Base), Data: types.PlatformAirGappedExport{
		PackageData: mapPlatformAirGappedPackage(finalized.Data), DownloadUrl: downloadURL, ExpiresAt: time.Now().Add(downloadTTL).Unix(),
	}}, nil
}

func importAirGappedResult(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ImportPlatformAirGappedResultReq) (*types.PlatformAirGappedPackageResp, error) {
	if req == nil || strings.TrimSpace(req.PackageCode) == "" || req.ResultObjectId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "packageCode and resultObjectId are required")
	}
	packageState, err := svcCtx.CoreCli.GetAirGappedPackage(ctx, &core.AirGappedPackageReq{PackageCode: req.PackageCode})
	if err != nil {
		return nil, err
	}
	resultObject, err := svcCtx.CoreCli.GetStorageObject(ctx, &core.StorageObjectIdReq{Id: req.ResultObjectId})
	if err != nil {
		return nil, err
	}
	if packageState.GetData() == nil || resultObject.GetData() == nil || resultObject.Data.AppId != packageState.Data.AppId ||
		resultObject.Data.ObjectType != core.StorageObjectType_STORAGE_OBJECT_TYPE_OFFLINE_RESULT_PACKAGE ||
		resultObject.Data.StorageMode != core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE || resultObject.Data.OwnerAgentId != 0 {
		return nil, status.Error(codes.FailedPrecondition, "AIR_GAPPED result object state or ownership is invalid")
	}
	if packageState.Data.Status == core.AirGappedPackageStatus_AIR_GAPPED_PACKAGE_STATUS_IMPORTED {
		if packageState.Data.ResultObjectId != resultObject.Data.Id || packageState.Data.ResultSha256 != resultObject.Data.Sha256 ||
			packageState.Data.ResultSizeBytes != resultObject.Data.SizeBytes ||
			resultObject.Data.Status != core.StorageObjectStatus_STORAGE_OBJECT_STATUS_BOUND {
			return nil, status.Error(codes.AlreadyExists, "AIR_GAPPED package was already imported with a different result object")
		}
		return &types.PlatformAirGappedPackageResp{RespBase: platformlogic.PlatformRespBase(packageState.Base),
			Data: mapPlatformAirGappedPackage(packageState.Data)}, nil
	}
	if resultObject.Data.Status != core.StorageObjectStatus_STORAGE_OBJECT_STATUS_READY {
		return nil, status.Error(codes.FailedPrecondition, "AIR_GAPPED result object is not ready")
	}
	store, err := platformlogic.LoadObjectStore(ctx, svcCtx)
	if err != nil {
		return nil, err
	}
	packageFile, err := copyVerifiedObjectToTemp(ctx, store, resultObject.Data)
	if err != nil {
		return nil, err
	}
	defer os.Remove(packageFile.Name())
	defer packageFile.Close()
	outputDir, err := os.MkdirTemp("", "appforge-air-gapped-result-")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create AIR_GAPPED output directory: %v", err)
	}
	defer os.RemoveAll(outputDir)
	outputPaths := map[string]string{}
	envelope, err := airgap.ReadResultPackage(packageFile, resultObject.Data.SizeBytes, func(artifact airgap.Artifact, reader io.Reader) error {
		name := ""
		switch artifact.Role {
		case "built_apk":
			name = "built.apk"
		case "build_log":
			name = "build.log"
		default:
			return status.Errorf(codes.InvalidArgument, "unsupported AIR_GAPPED output role %q", artifact.Role)
		}
		filename := filepath.Join(outputDir, name)
		file, createErr := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if createErr != nil {
			return createErr
		}
		written, copyErr := io.Copy(file, reader)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if written != artifact.SizeBytes {
			return status.Error(codes.FailedPrecondition, "AIR_GAPPED output size changed while extracting")
		}
		outputPaths[artifact.Role] = filename
		return nil
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "verify AIR_GAPPED result package: %v", err)
	}
	if envelope.Manifest.PackageCode != packageState.Data.PackageCode {
		return nil, status.Error(codes.PermissionDenied, "AIR_GAPPED result package code mismatch")
	}
	created := make([]*core.StorageObject, 0, 2)
	cleanup := func() {
		for _, object := range created {
			platformlogic.CleanupFailedUpload(ctx, svcCtx, store, object)
		}
	}
	var apkID, logID int64
	for _, artifact := range envelope.Manifest.Outputs {
		objectType, contentType, filename := core.StorageObjectType_STORAGE_OBJECT_TYPE_UNKNOWN, artifact.ContentType, artifact.OriginalName
		switch artifact.Role {
		case "built_apk":
			objectType, filename = core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK, "built.apk"
		case "build_log":
			objectType, filename = core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG, "build.log"
		}
		file, err := os.Open(outputPaths[artifact.Role])
		if err != nil {
			cleanup()
			return nil, err
		}
		object, putErr := putVerifiedFile(ctx, svcCtx, store, packageState.Data.AppId, objectType, filename, contentType, file)
		_ = file.Close()
		if putErr != nil {
			cleanup()
			return nil, putErr
		}
		created = append(created, object)
		if artifact.Role == "built_apk" {
			apkID = object.Id
		} else {
			logID = object.Id
		}
	}
	manifestJSON, err := airgap.CanonicalJSON(envelope.Manifest)
	if err != nil {
		cleanup()
		return nil, err
	}
	imported, err := svcCtx.CoreCli.ImportAirGappedResult(ctx, &core.ImportAirGappedResultReq{
		PackageCode: req.PackageCode, ResultObjectId: resultObject.Data.Id, ResultSha256: resultObject.Data.Sha256,
		ResultSizeBytes: resultObject.Data.SizeBytes, ResultManifestJson: string(manifestJSON), AgentCertificatePem: envelope.CertificatePEM,
		SignatureAlgorithm: envelope.Signature.Algorithm, SignatureBase64: envelope.Signature.Value, ApkObjectId: apkID, LogObjectId: logID,
	})
	if err != nil {
		cleanup()
		return nil, err
	}
	return &types.PlatformAirGappedPackageResp{RespBase: platformlogic.PlatformRespBase(imported.Base), Data: mapPlatformAirGappedPackage(imported.Data)}, nil
}

func buildAirGappedTaskBundle(execution *core.BuildExecutionContext) (*airGappedBuildManifest, []airgap.Artifact, map[int64]*core.StorageObject, error) {
	if execution == nil || execution.Task == nil || execution.Task.Id <= 0 || strings.TrimSpace(execution.SecretRef) == "" ||
		!strings.HasPrefix(strings.ToLower(strings.TrimSpace(execution.SecretRef)), "local-file://") {
		return nil, nil, nil, status.Error(codes.FailedPrecondition, "AIR_GAPPED execution context or local signing Secret is unavailable")
	}
	if strings.Contains(execution.TemplateSnapshotJson, "sb1.") {
		return nil, nil, nil, status.Error(codes.FailedPrecondition, "AIR_GAPPED does not expose control-plane encrypted template parameters")
	}
	task := execution.Task
	bundle := &airGappedBuildManifest{SchemaVersion: 3, Task: &airGappedBuildTask{ID: task.Id, TenantID: task.TenantId,
		AppID: task.AppId, VersionID: task.VersionId, BuilderAttempt: task.BuilderAttempt, ChannelCode: task.ChannelCode,
		VersionCode: task.VersionCode, VersionName: task.VersionName}, PackageName: execution.PackageName,
		APIHost: execution.ApiHost, ChannelName: execution.ChannelName, LandingURL: execution.LandingUrl,
		KeyAlias: execution.KeyAlias, SigningSecretRef: execution.SecretRef, SignerCertificateSHA256: execution.SignerCertificateSha256,
		BrandingSnapshotJSON: execution.BrandingSnapshotJson, TemplateSnapshotJSON: execution.TemplateSnapshotJson}
	artifacts := make([]airgap.Artifact, 0, 4+len(execution.TemplateFiles))
	objects := map[int64]*core.StorageObject{}
	appendInput := func(role, artifactPath string, object *core.StorageObject, required bool) error {
		if object == nil || object.Id <= 0 {
			if required {
				return status.Errorf(codes.FailedPrecondition, "AIR_GAPPED %s input is unavailable", role)
			}
			return nil
		}
		if object.TenantId != execution.Task.TenantId || object.AppId != execution.Task.AppId || object.SizeBytes <= 0 || len(object.Sha256) != 64 ||
			object.StorageMode != core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE || object.OwnerAgentId != 0 ||
			(object.Status != core.StorageObjectStatus_STORAGE_OBJECT_STATUS_READY && object.Status != core.StorageObjectStatus_STORAGE_OBJECT_STATUS_BOUND) {
			return status.Errorf(codes.FailedPrecondition, "AIR_GAPPED %s input metadata or ownership is invalid", role)
		}
		input := airGappedBuildInput{Role: role, ObjectID: object.Id, ObjectType: object.ObjectType, OriginalName: object.OriginalName,
			ContentType: object.ContentType, SizeBytes: object.SizeBytes, SHA256: object.Sha256}
		bundle.Inputs = append(bundle.Inputs, input)
		artifacts = append(artifacts, airgap.Artifact{Role: role, Path: artifactPath, ObjectID: object.Id, ObjectType: int32(object.ObjectType),
			OriginalName: object.OriginalName, ContentType: object.ContentType, SizeBytes: object.SizeBytes, SHA256: object.Sha256})
		objects[object.Id] = object
		return nil
	}
	if err := appendInput("source_apk", "inputs/source.apk", execution.SourceApk, true); err != nil {
		return nil, nil, nil, err
	}
	keystoreExt := strings.ToLower(filepath.Ext(execution.Keystore.GetOriginalName()))
	if keystoreExt == "" || len(keystoreExt) > 16 {
		return nil, nil, nil, status.Error(codes.FailedPrecondition, "AIR_GAPPED Keystore extension is invalid")
	}
	if err := appendInput("keystore", "inputs/signing"+keystoreExt, execution.Keystore, true); err != nil {
		return nil, nil, nil, err
	}
	if err := appendInput("brand_logo", "inputs/brand-logo"+strings.ToLower(filepath.Ext(execution.BrandLogo.GetOriginalName())), execution.BrandLogo, false); err != nil {
		return nil, nil, nil, err
	}
	if err := appendInput("brand_splash", "inputs/brand-splash"+strings.ToLower(filepath.Ext(execution.BrandSplash.GetOriginalName())), execution.BrandSplash, false); err != nil {
		return nil, nil, nil, err
	}
	for index, object := range execution.TemplateFiles {
		extension := strings.ToLower(filepath.Ext(object.GetOriginalName()))
		if extension == "" || len(extension) > 16 {
			return nil, nil, nil, status.Error(codes.FailedPrecondition, "AIR_GAPPED template file extension is invalid")
		}
		if err := appendInput("template_file", fmt.Sprintf("inputs/templates/%03d%s", index, extension), object, true); err != nil {
			return nil, nil, nil, err
		}
	}
	return bundle, artifacts, objects, nil
}

func putVerifiedFile(ctx context.Context, svcCtx *svc.ServiceContext, store storage.ObjectStore, appID int64, objectType core.StorageObjectType,
	filename, contentType string, file *os.File) (*core.StorageObject, error) {
	if file == nil {
		return nil, status.Error(codes.InvalidArgument, "source file is required")
	}
	info, err := file.Stat()
	if err != nil || info.Size() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "source file is empty or unavailable")
	}
	objectKey, err := platformlogic.GenerateStorageObjectKey(ctx, objectType, filename)
	if err != nil {
		return nil, err
	}
	created, err := svcCtx.CoreCli.CreateStorageObject(ctx, &core.CreateStorageObjectReq{AppId: appID, ObjectType: objectType,
		ObjectKey: objectKey, OriginalName: filename, ContentType: contentType, SizeBytes: info.Size()})
	if err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		platformlogic.CleanupFailedUpload(ctx, svcCtx, store, created.Data)
		return nil, err
	}
	if err := store.PutObject(ctx, objectKey, file, info.Size(), contentType); err != nil {
		platformlogic.CleanupFailedUpload(ctx, svcCtx, store, created.Data)
		return nil, status.Errorf(codes.Internal, "upload AIR_GAPPED object: %v", err)
	}
	size, digest, err := platformlogic.VerifyStorageObject(ctx, store, created.Data)
	if err != nil {
		platformlogic.CleanupFailedUpload(ctx, svcCtx, store, created.Data)
		return nil, err
	}
	completed, err := svcCtx.CoreCli.CompleteStorageObject(ctx, &core.CompleteStorageObjectReq{Id: created.Data.Id, SizeBytes: size, Sha256: digest})
	if err != nil {
		platformlogic.CleanupFailedUpload(ctx, svcCtx, store, created.Data)
		return nil, err
	}
	return completed.Data, nil
}

func copyVerifiedObjectToTemp(ctx context.Context, store storage.ObjectStore, object *core.StorageObject) (*os.File, error) {
	reader, err := store.OpenObject(ctx, object.ObjectKey)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "open AIR_GAPPED result object: %v", err)
	}
	defer reader.Close()
	file, err := os.CreateTemp("", "appforge-air-gapped-import-*.zip")
	if err != nil {
		return nil, err
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(reader, object.SizeBytes+1))
	if copyErr != nil || written != object.SizeBytes || hex.EncodeToString(hasher.Sum(nil)) != object.Sha256 {
		file.Close()
		os.Remove(file.Name())
		return nil, status.Error(codes.FailedPrecondition, "AIR_GAPPED result object bytes differ from completed metadata")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		os.Remove(file.Name())
		return nil, err
	}
	return file, nil
}

func mapPlatformAirGappedPackage(item *core.AirGappedPackage) types.PlatformAirGappedPackage {
	if item == nil {
		return types.PlatformAirGappedPackage{}
	}
	return types.PlatformAirGappedPackage{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, PackageCode: item.PackageCode,
		AgentId: item.AgentId, TaskId: item.TaskId, BuilderAttempt: item.BuilderAttempt, AgentCertificateSerial: item.AgentCertificateSerial,
		Status: int32(item.Status), ExportObjectId: item.ExportObjectId, ExportSha256: item.ExportSha256, ExportSizeBytes: item.ExportSizeBytes,
		ResultObjectId: item.ResultObjectId, ResultSha256: item.ResultSha256, ResultSizeBytes: item.ResultSizeBytes,
		IssuedAt: item.IssuedAt, ExpiresAt: item.ExpiresAt, ImportedAt: item.ImportedAt, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}
