package logic

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/common/storage"
	"appforge/common/utils"
	"appforge/proto/core"
	"appforge/proto/system"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	maxSourceAPKBytes = int64(2 * 1024 * 1024 * 1024)
	maxKeystoreBytes  = int64(10 * 1024 * 1024)
)

// LoadObjectStore loads system-level storage credentials for internal use only.
func LoadObjectStore(ctx context.Context, svcCtx *svc.ServiceContext) (storage.ObjectStore, error) {
	configKey := system.SysConfigType_OBJECT_STORAGE
	tenantID := int64(0)
	result, err := svcCtx.SystemCli.SysConfigDetail(ctx, &system.SysConfigDetailReq{
		ConfigKey: &configKey,
		TenantId:  &tenantID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load object storage config failed: %v", err)
	}
	if result == nil || result.Data == nil {
		return nil, status.Error(codes.FailedPrecondition, "object storage config is missing")
	}
	var cfg storage.Config
	if err := json.Unmarshal([]byte(result.Data.ConfigValue), &cfg); err != nil {
		return nil, status.Errorf(codes.Internal, "decode object storage config failed: %v", err)
	}
	store, err := storage.NewObjectStore(cfg)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "create object store failed: %v", err)
	}
	return store, nil
}

// TenantID returns the authenticated tenant boundary used by Core RPC.
func TenantID(ctx context.Context) (int64, error) {
	value, err := utils.GetTenantIdFromCtx(ctx)
	if err != nil || value <= 0 {
		return 0, status.Error(codes.Unauthenticated, "tenant context is required")
	}
	return value, nil
}

// GenerateStorageObjectKey creates a key inside the authenticated tenant namespace.
func GenerateStorageObjectKey(ctx context.Context, objectType core.StorageObjectType, filename string) (string, error) {
	tenantID, err := TenantID(ctx)
	if err != nil {
		return "", err
	}
	prefix := ""
	switch objectType {
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK:
		prefix = "source-apk"
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE:
		prefix = "keystore"
	default:
		return "", status.Error(codes.InvalidArgument, "upload object type is not supported")
	}
	key, err := storage.GenerateTenantObjectKey(tenantID, prefix, filename)
	if err != nil {
		return "", status.Errorf(codes.Internal, "generate object key failed: %v", err)
	}
	return key, nil
}

// VerifyStorageObject streams one private object to a temporary file, computes SHA-256,
// and validates the minimum file structure required by V1.
func VerifyStorageObject(ctx context.Context, store storage.ObjectStore, item *core.StorageObject) (int64, string, error) {
	if item == nil {
		return 0, "", status.Error(codes.InvalidArgument, "storage object is required")
	}
	info, err := store.StatObject(ctx, item.ObjectKey)
	if err != nil {
		return 0, "", status.Errorf(codes.FailedPrecondition, "uploaded object is missing: %v", err)
	}
	if info.Size <= 0 || info.Size != item.SizeBytes {
		return 0, "", status.Error(codes.FailedPrecondition, "uploaded object size does not match declaration")
	}
	if item.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK && info.Size > maxSourceAPKBytes {
		return 0, "", status.Error(codes.InvalidArgument, "source APK exceeds 2 GiB")
	}
	if item.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE && info.Size > maxKeystoreBytes {
		return 0, "", status.Error(codes.InvalidArgument, "keystore exceeds 10 MiB")
	}

	reader, err := store.OpenObject(ctx, item.ObjectKey)
	if err != nil {
		return 0, "", status.Errorf(codes.Internal, "open uploaded object failed: %v", err)
	}
	defer reader.Close()

	tempFile, err := os.CreateTemp("", "appforge-upload-*")
	if err != nil {
		return 0, "", status.Errorf(codes.Internal, "create upload verification file failed: %v", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tempFile, hasher), io.LimitReader(reader, info.Size+1))
	closeErr := tempFile.Close()
	if copyErr != nil {
		return 0, "", status.Errorf(codes.Internal, "read uploaded object failed: %v", copyErr)
	}
	if closeErr != nil {
		return 0, "", status.Errorf(codes.Internal, "close upload verification file failed: %v", closeErr)
	}
	if written != info.Size {
		return 0, "", status.Error(codes.FailedPrecondition, "uploaded object changed during verification")
	}

	switch item.ObjectType {
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK:
		if err := verifyAPK(tempPath); err != nil {
			return 0, "", err
		}
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE:
		if err := verifyKeystoreHeader(tempPath); err != nil {
			return 0, "", err
		}
	default:
		return 0, "", status.Error(codes.InvalidArgument, "upload object type is not supported")
	}
	return written, hex.EncodeToString(hasher.Sum(nil)), nil
}

func verifyAPK(filename string) error {
	reader, err := zip.OpenReader(filename)
	if err != nil {
		return status.Error(codes.InvalidArgument, "source APK is not a valid ZIP archive")
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name == "AndroidManifest.xml" && file.UncompressedSize64 > 0 {
			return nil
		}
	}
	return status.Error(codes.InvalidArgument, "source APK does not contain AndroidManifest.xml")
}

func verifyKeystoreHeader(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return status.Errorf(codes.Internal, "open keystore verification file failed: %v", err)
	}
	defer file.Close()
	header := make([]byte, 4)
	if _, err := io.ReadFull(file, header); err != nil {
		return status.Error(codes.InvalidArgument, "keystore file is too small")
	}
	magic := hex.EncodeToString(header)
	if magic == "feedfeed" || magic == "cececece" || strings.HasPrefix(magic, "30") {
		return nil
	}
	return status.Error(codes.InvalidArgument, "keystore file header is not JKS, JCEKS or PKCS12")
}

// MapPlatformStorageObject maps internal metadata without exposing object keys.
func MapPlatformStorageObject(item *core.StorageObject) types.PlatformStorageObject {
	if item == nil {
		return types.PlatformStorageObject{}
	}
	return types.PlatformStorageObject{
		ObjectId: item.Id, AppId: item.AppId, ObjectType: int32(item.ObjectType),
		OriginalName: item.OriginalName, SizeBytes: item.SizeBytes, Sha256: item.Sha256,
		Status: int32(item.Status),
	}
}

// CleanupFailedUpload removes the physical object and marks metadata failed.
func CleanupFailedUpload(ctx context.Context, svcCtx *svc.ServiceContext, store storage.ObjectStore, item *core.StorageObject) {
	if item == nil {
		return
	}
	if err := store.DeleteObject(ctx, item.ObjectKey); err != nil {
		logStorageCleanupError(ctx, item.Id, err)
	}
	if _, err := svcCtx.CoreCli.FailStorageObject(ctx, &core.FailStorageObjectReq{Id: item.Id}); err != nil {
		logStorageCleanupError(ctx, item.Id, err)
	}
}

func logStorageCleanupError(ctx context.Context, objectID int64, err error) {
	logx.WithContext(ctx).Errorf("storage object %d cleanup failed: %v", objectID, err)
}
