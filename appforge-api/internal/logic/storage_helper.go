package logic

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

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
	maxSourceAPKBytes    = int64(2 * 1024 * 1024 * 1024)
	maxKeystoreBytes     = int64(10 * 1024 * 1024)
	maxBrandLogoBytes    = int64(5 * 1024 * 1024)
	maxBrandSplashBytes  = int64(10 * 1024 * 1024)
	maxTemplateFileBytes = int64(2 * 1024 * 1024)
	maxBuiltAPKBytes     = int64(2 * 1024 * 1024 * 1024)
	maxBuildLogBytes     = int64(100 * 1024 * 1024)
	maxOfflinePackBytes  = int64(3 * 1024 * 1024 * 1024)
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
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO:
		prefix = "brand-logo"
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH:
		prefix = "brand-splash"
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_TEMPLATE_FILE:
		prefix = "template-file"
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK:
		prefix = "built-apk"
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG:
		prefix = "build-log"
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_OFFLINE_TASK_PACKAGE:
		prefix = "offline-task"
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_OFFLINE_RESULT_PACKAGE:
		prefix = "offline-result"
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
	if item.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO && info.Size > maxBrandLogoBytes {
		return 0, "", status.Error(codes.InvalidArgument, "brand logo exceeds 5 MiB")
	}
	if item.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH && info.Size > maxBrandSplashBytes {
		return 0, "", status.Error(codes.InvalidArgument, "brand splash exceeds 10 MiB")
	}
	if item.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_TEMPLATE_FILE && info.Size > maxTemplateFileBytes {
		return 0, "", status.Error(codes.InvalidArgument, "template file exceeds 2 MiB")
	}
	if item.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK && info.Size > maxBuiltAPKBytes {
		return 0, "", status.Error(codes.InvalidArgument, "built APK exceeds 2 GiB")
	}
	if item.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG && info.Size > maxBuildLogBytes {
		return 0, "", status.Error(codes.InvalidArgument, "build log exceeds 100 MiB")
	}
	if (item.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_OFFLINE_TASK_PACKAGE ||
		item.ObjectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_OFFLINE_RESULT_PACKAGE) && info.Size > maxOfflinePackBytes {
		return 0, "", status.Error(codes.InvalidArgument, "AIR_GAPPED package exceeds 3 GiB")
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
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO,
		core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH:
		if err := verifyBrandingImage(tempPath, item.ObjectType); err != nil {
			return 0, "", err
		}
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_TEMPLATE_FILE:
		if err := verifyTemplateFile(tempPath, item.OriginalName); err != nil {
			return 0, "", err
		}
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK:
		if err := verifyAPK(tempPath); err != nil {
			return 0, "", err
		}
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG:
		data, err := os.ReadFile(tempPath)
		if err != nil || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return 0, "", status.Error(codes.InvalidArgument, "build log must be NUL-free UTF-8")
		}
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_OFFLINE_TASK_PACKAGE,
		core.StorageObjectType_STORAGE_OBJECT_TYPE_OFFLINE_RESULT_PACKAGE:
		archive, err := zip.OpenReader(tempPath)
		if err != nil || len(archive.File) == 0 || len(archive.File) > 260 {
			if archive != nil {
				_ = archive.Close()
			}
			return 0, "", status.Error(codes.InvalidArgument, "AIR_GAPPED package is not a bounded ZIP")
		}
		_ = archive.Close()
	default:
		return 0, "", status.Error(codes.InvalidArgument, "upload object type is not supported")
	}
	return written, hex.EncodeToString(hasher.Sum(nil)), nil
}

func verifyTemplateFile(filename, originalName string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return status.Errorf(codes.Internal, "read template file failed: %v", err)
	}
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(originalName))) {
	case ".json":
		if !json.Valid(data) {
			return status.Error(codes.InvalidArgument, "template JSON file is invalid")
		}
		var value any
		if err := json.Unmarshal(data, &value); err != nil || containsTemplateSecret(value) {
			return status.Error(codes.InvalidArgument, "template files cannot contain plaintext secrets; use a sensitive template parameter")
		}
	case ".xml":
		decoder := xml.NewDecoder(bytes.NewReader(data))
		for {
			token, decodeErr := decoder.Token()
			if decodeErr == io.EOF {
				break
			}
			if decodeErr != nil {
				return status.Error(codes.InvalidArgument, "template XML file is invalid")
			}
			if _, forbidden := token.(xml.Directive); forbidden {
				return status.Error(codes.InvalidArgument, "template XML directives are not allowed")
			}
		}
	case ".txt":
		if !utf8.Valid(data) {
			return status.Error(codes.InvalidArgument, "template TXT file must be UTF-8")
		}
	case ".png":
		if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
			return status.Error(codes.InvalidArgument, "template PNG file is invalid")
		}
	case ".webp":
		if _, _, err := webPDimensions(data); err != nil {
			return err
		}
	default:
		return status.Error(codes.InvalidArgument, "template file type is not supported")
	}
	if containsSecretText(string(data)) {
		return status.Error(codes.InvalidArgument, "template files cannot contain plaintext secrets; use a sensitive template parameter")
	}
	return nil
}

func containsTemplateSecret(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(key))
			if normalized == "clientsecret" || normalized == "privatekey" || normalized == "password" ||
				normalized == "accesstoken" || normalized == "refreshtoken" {
				return true
			}
			if containsTemplateSecret(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsTemplateSecret(child) {
				return true
			}
		}
	case string:
		return containsSecretText(typed)
	}
	return false
}

func containsSecretText(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"-----begin private key", "-----begin rsa private key", "client_secret", "<clientsecret", "<password", "access_token", "refresh_token"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func verifyBrandingImage(filename string, objectType core.StorageObjectType) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return status.Errorf(codes.Internal, "read branding image failed: %v", err)
	}
	var width, height int
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")):
		config, decodeErr := png.DecodeConfig(bytes.NewReader(data))
		if decodeErr != nil {
			return status.Error(codes.InvalidArgument, "branding PNG is invalid")
		}
		width, height = config.Width, config.Height
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		width, height, err = webPDimensions(data)
		if err != nil {
			return err
		}
	default:
		return status.Error(codes.InvalidArgument, "branding image must be a real PNG or WebP file")
	}
	if objectType == core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO {
		if width != height || width < 512 || width > 2048 {
			return status.Error(codes.InvalidArgument, "brand logo must be square and between 512x512 and 2048x2048")
		}
		return nil
	}
	if width < 720 || height < 720 {
		return status.Error(codes.InvalidArgument, "brand splash shortest side must be at least 720 pixels")
	}
	return nil
}

func webPDimensions(data []byte) (int, int, error) {
	width, height := 0, 0
	for offset := 12; offset+8 <= len(data); {
		chunkType := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		start := offset + 8
		end := start + size
		if size < 0 || end < start || end > len(data) {
			return 0, 0, status.Error(codes.InvalidArgument, "branding WebP has an invalid chunk")
		}
		chunk := data[start:end]
		switch chunkType {
		case "ANIM", "ANMF":
			return 0, 0, status.Error(codes.InvalidArgument, "animated WebP is not supported")
		case "VP8X":
			if len(chunk) < 10 {
				return 0, 0, status.Error(codes.InvalidArgument, "branding WebP VP8X header is invalid")
			}
			if chunk[0]&0x02 != 0 {
				return 0, 0, status.Error(codes.InvalidArgument, "animated WebP is not supported")
			}
			width = 1 + int(chunk[4]) + int(chunk[5])<<8 + int(chunk[6])<<16
			height = 1 + int(chunk[7]) + int(chunk[8])<<8 + int(chunk[9])<<16
		case "VP8 ":
			if len(chunk) < 10 || !bytes.Equal(chunk[3:6], []byte{0x9d, 0x01, 0x2a}) {
				return 0, 0, status.Error(codes.InvalidArgument, "branding WebP VP8 header is invalid")
			}
			width = int(binary.LittleEndian.Uint16(chunk[6:8]) & 0x3fff)
			height = int(binary.LittleEndian.Uint16(chunk[8:10]) & 0x3fff)
		case "VP8L":
			if len(chunk) < 5 || chunk[0] != 0x2f {
				return 0, 0, status.Error(codes.InvalidArgument, "branding WebP VP8L header is invalid")
			}
			width = 1 + int(chunk[1]) + (int(chunk[2]&0x3f) << 8)
			height = 1 + int(chunk[2]>>6) + (int(chunk[3]) << 2) + (int(chunk[4]&0x0f) << 10)
		}
		offset = end + size%2
	}
	if width <= 0 || height <= 0 {
		return 0, 0, status.Error(codes.InvalidArgument, "branding WebP dimensions are unavailable")
	}
	return width, height, nil
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
		Status: int32(item.Status), StorageMode: int32(item.StorageMode), OwnerAgentId: item.OwnerAgentId,
	}
}

// CleanupFailedUpload removes the physical object and marks metadata failed.
func CleanupFailedUpload(ctx context.Context, svcCtx *svc.ServiceContext, store storage.ObjectStore, item *core.StorageObject) {
	if item == nil {
		return
	}
	if item.StorageMode != core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE || item.OwnerAgentId != 0 {
		logStorageCleanupError(ctx, item.Id, errors.New("refusing to delete non-control-plane storage object"))
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
