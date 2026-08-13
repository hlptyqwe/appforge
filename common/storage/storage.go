package storage

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"

	"appforge/common/i18n"
	"appforge/common/storage/impl"
)

// OssType indicates which implementation will be used for uploads.
//
// These numeric values are aligned with the existing protobuf definitions.
// Keep them stable to avoid breaking stored configurations.
type OssType int64

const (
	OssTypeAliyun  OssType = 1
	OssTypeTencent OssType = 2
	OssTypeMinio   OssType = 3
)

// Common error messages for configuration validation.
var (
	ErrUnsupportedOssType = errors.New("unsupported oss type")
)

// UploadFile is a convenience wrapper around NewUploader.
// It keeps the old call signature while leveraging the interface-based implementation.
func UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, cfg Config) (string, error) {
	uploader, err := NewUploader(ctx, cfg)
	if err != nil {
		return "", err
	}
	return uploader.Upload(ctx, file, header)
}

// Uploader is responsible for uploading a file and returning a relative path.
// The caller can choose how to combine the returned value with an optional domain.
type Uploader interface {
	Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error)
}

// NewUploader creates an uploader for the configured provider.
func NewUploader(ctx context.Context, cfg Config) (Uploader, error) {
	switch cfg.OssType {
	case OssTypeAliyun:
		return impl.NewAliyunUploader(cfg.AliyunOss.Endpoint, cfg.AliyunOss.AccessKeyId, cfg.AliyunOss.AccessKeySecret, cfg.AliyunOss.BucketName)
	case OssTypeTencent:
		return impl.NewTencentUploader(cfg.TencentCos.Market, cfg.TencentCos.SecretId, cfg.TencentCos.SecretKey, cfg.TencentCos.BucketName, cfg.TencentCos.BucketUrl)
	case OssTypeMinio:
		return impl.NewMinioUploader(cfg.Minio.Endpoint, cfg.Minio.AccessKeyId, cfg.Minio.AccessKeySecret, cfg.Minio.BucketName)
	default:
		fmt.Printf("不支持的对象存储类型 %d\n", cfg.OssType)
		return nil, i18n.StatusError(ctx, i18n.InternalServerError)
	}
}

// Config holds required connection information for each storage provider.
// This type is deliberately kept provider-agnostic so that the common/storage
// package does not depend on any external proto types.
type Config struct {
	AliyunOss  *AliyunOssConfig  `json:"aliyun_oss,omitempty"`
	TencentCos *TencentCosConfig `json:"tencent_cos,omitempty"`
	Minio      *MinioConfig      `json:"minio,omitempty"`
	OssType    OssType           `json:"oss_type,omitempty"`   // 1阿里云OSS 2腾讯云COS 3MINIO
	OssDomain  string            `json:"oss_domain,omitempty"` // 对象存储访问域名（可选，优先使用bucket_url）
}

// AliyunOssConfig contains the subset of OSS config required for uploads.
type AliyunOssConfig struct {
	Endpoint        string `json:"endpoint,omitempty"`
	AccessKeyId     string `json:"access_key_id,omitempty"`
	AccessKeySecret string `json:"access_key_secret,omitempty"`
	BucketName      string `json:"bucket_name,omitempty"`
	BucketUrl       string `json:"bucket_url,omitempty"`
}

// TencentCosConfig contains the subset of COS config required for uploads.
type TencentCosConfig struct {
	Market     string `json:"market,omitempty"`
	SecretId   string `json:"secret_id,omitempty"`
	SecretKey  string `json:"secret_key,omitempty"`
	BucketName string `json:"bucket_name,omitempty"`
	BucketUrl  string `json:"bucket_url,omitempty"`
}

// MinioConfig contains the subset of MinIO config required for uploads.
type MinioConfig struct {
	Endpoint        string `json:"endpoint,omitempty"`
	AccessKeyId     string `json:"access_key_id,omitempty"`
	AccessKeySecret string `json:"access_key_secret,omitempty"`
	BucketName      string `json:"bucket_name,omitempty"`
	BucketUrl       string `json:"bucket_url,omitempty"`
}
