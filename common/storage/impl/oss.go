package impl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"

	"appforge/common/storage/internal"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type aliyunUploader struct {
	endpoint        string
	accessKeyID     string
	accessKeySecret string
	bucketName      string
}

func NewAliyunUploader(endpoint, accessKeyID, accessKeySecret, bucketName string) (*aliyunUploader, error) {
	if endpoint == "" || accessKeyID == "" || accessKeySecret == "" || bucketName == "" {
		return nil, fmt.Errorf("aliyun uploader missing required parameters")
	}
	return &aliyunUploader{
		endpoint:        endpoint,
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		bucketName:      bucketName,
	}, nil
}

func (u *aliyunUploader) Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	key := internal.GenerateObjectKey(header.Filename)
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	client, err := oss.New(u.endpoint, u.accessKeyID, u.accessKeySecret)
	if err != nil {
		return "", err
	}

	bucketName := u.bucketName
	// Ensure bucket exists and is publicly readable.
	exists, err := client.IsBucketExist(bucketName)
	if err != nil {
		return "", err
	}
	if !exists {
		if err = client.CreateBucket(bucketName, oss.ACL(oss.ACLPublicRead)); err != nil {
			return "", err
		}
	}
	if err = client.SetBucketACL(bucketName, oss.ACLPublicRead); err != nil {
		return "", err
	}

	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return "", err
	}

	options := []oss.Option{oss.ContentType(contentType)}
	if err = bucket.PutObject(key, bytes.NewReader(data), options...); err != nil {
		return "", err
	}

	return "/" + key, nil
}
