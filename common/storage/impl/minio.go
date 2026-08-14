package impl

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"path"
	"strings"
	"time"

	"appforge/common/storage/internal"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioUploader struct {
	endpoint        string
	publicEndpoint  string
	accessKeyID     string
	accessKeySecret string
	bucketName      string
}

func NewMinioUploader(endpoint, accessKeyID, accessKeySecret, bucketName string) (*minioUploader, error) {
	return NewMinioObjectStore(endpoint, endpoint, accessKeyID, accessKeySecret, bucketName)
}

// NewMinioObjectStore creates a private MinIO object store. publicEndpoint may be a bucket URL
// reachable by browsers; endpoint remains the internal service address used by backend services.
func NewMinioObjectStore(endpoint, publicEndpoint, accessKeyID, accessKeySecret, bucketName string) (*minioUploader, error) {
	if endpoint == "" || accessKeyID == "" || accessKeySecret == "" || bucketName == "" {
		return nil, fmt.Errorf("minio uploader missing required parameters")
	}
	publicEndpoint = normalizePublicEndpoint(publicEndpoint, bucketName)
	if publicEndpoint == "" {
		publicEndpoint = endpoint
	}
	return &minioUploader{
		endpoint:        endpoint,
		publicEndpoint:  publicEndpoint,
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		bucketName:      bucketName,
	}, nil
}

func (u *minioUploader) Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error) {
	key := internal.GenerateObjectKey(header.Filename)
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := u.PutObject(ctx, key, file, header.Size, contentType); err != nil {
		return "", err
	}
	return path.Join("/", u.bucketName, key), nil
}

func (u *minioUploader) PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	if strings.TrimSpace(key) == "" || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return fmt.Errorf("invalid object key")
	}
	if size < 0 {
		return fmt.Errorf("invalid object size")
	}
	client, err := u.client(u.endpoint)
	if err != nil {
		return err
	}
	if err := u.ensureBucket(ctx, client); err != nil {
		return err
	}
	_, err = client.PutObject(ctx, u.bucketName, key, reader, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (u *minioUploader) OpenObject(ctx context.Context, key string) (io.ReadCloser, error) {
	client, err := u.client(u.endpoint)
	if err != nil {
		return nil, err
	}
	object, err := client.GetObject(ctx, u.bucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := object.Stat(); err != nil {
		_ = object.Close()
		return nil, err
	}
	return object, nil
}

func (u *minioUploader) StatObject(ctx context.Context, key string) (internal.ObjectInfo, error) {
	client, err := u.client(u.endpoint)
	if err != nil {
		return internal.ObjectInfo{}, err
	}
	info, err := client.StatObject(ctx, u.bucketName, key, minio.StatObjectOptions{})
	if err != nil {
		return internal.ObjectInfo{}, err
	}
	return internal.ObjectInfo{Key: key, Size: info.Size, ContentType: info.ContentType}, nil
}

func (u *minioUploader) DeleteObject(ctx context.Context, key string) error {
	client, err := u.client(u.endpoint)
	if err != nil {
		return err
	}
	return client.RemoveObject(ctx, u.bucketName, key, minio.RemoveObjectOptions{})
}

func (u *minioUploader) PresignPut(ctx context.Context, key string, expires time.Duration) (string, error) {
	client, err := u.client(u.publicEndpoint)
	if err != nil {
		return "", err
	}
	signed, err := client.PresignedPutObject(ctx, u.bucketName, key, expires)
	if err != nil {
		return "", err
	}
	return signed.String(), nil
}

func (u *minioUploader) PresignGet(ctx context.Context, key string, expires time.Duration) (string, error) {
	client, err := u.client(u.publicEndpoint)
	if err != nil {
		return "", err
	}
	signed, err := client.PresignedGetObject(ctx, u.bucketName, key, expires, nil)
	if err != nil {
		return "", err
	}
	return signed.String(), nil
}

func (u *minioUploader) client(rawEndpoint string) (*minio.Client, error) {
	endpoint := strings.TrimSpace(rawEndpoint)
	// 未显式配置协议时按 MinIO 常用的内网 HTTP 地址处理；生产环境如需
	// TLS，必须显式配置 https://，避免类似 minio:9000 被误判为 HTTPS。
	secure := false
	if strings.HasPrefix(endpoint, "http://") {
		secure = false
		endpoint = strings.TrimPrefix(endpoint, "http://")
	} else if strings.HasPrefix(endpoint, "https://") {
		secure = true
		endpoint = strings.TrimPrefix(endpoint, "https://")
	}

	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(u.accessKeyID, u.accessKeySecret, ""),
		Secure: secure,
		Region: "us-east-1",
	})
}

func (u *minioUploader) ensureBucket(ctx context.Context, client *minio.Client) error {
	found, err := client.BucketExists(ctx, u.bucketName)
	if err != nil {
		return err
	}
	if !found {
		if err := client.MakeBucket(ctx, u.bucketName, minio.MakeBucketOptions{Region: "us-east-1"}); err != nil {
			return err
		}
	}
	// An empty policy keeps the bucket private and removes legacy anonymous-read policy.
	return client.SetBucketPolicy(ctx, u.bucketName, "")
}

func normalizePublicEndpoint(value, bucketName string) string {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return value
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/"+strings.Trim(bucketName, "/"))
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimSuffix(parsed.String(), "/")
}
