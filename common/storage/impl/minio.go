package impl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	"appforge/common/storage/internal"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioUploader struct {
	endpoint        string
	accessKeyID     string
	accessKeySecret string
	bucketName      string
}

func NewMinioUploader(endpoint, accessKeyID, accessKeySecret, bucketName string) (*minioUploader, error) {
	if endpoint == "" || accessKeyID == "" || accessKeySecret == "" || bucketName == "" {
		return nil, fmt.Errorf("minio uploader missing required parameters")
	}
	return &minioUploader{
		endpoint:        endpoint,
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		bucketName:      bucketName,
	}, nil
}

func (u *minioUploader) Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	key := internal.GenerateObjectKey(header.Filename)
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	endpoint := strings.TrimSpace(u.endpoint)
	secure := true
	if strings.HasPrefix(endpoint, "http://") {
		secure = false
		endpoint = strings.TrimPrefix(endpoint, "http://")
	} else if strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimPrefix(endpoint, "https://")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(u.accessKeyID, u.accessKeySecret, ""),
		Secure: secure,
	})
	if err != nil {
		return "", err
	}

	bucketName := u.bucketName
	found, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return "", err
	}
	if !found {
		if err := client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
			return "", err
		}
	}

	// Ensure public-read policy
	policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucketName)
	if err := client.SetBucketPolicy(ctx, bucketName, policy); err != nil {
		return "", err
	}

	_, err = client.PutObject(ctx, bucketName, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("/%s/%s", bucketName, key), nil
}
