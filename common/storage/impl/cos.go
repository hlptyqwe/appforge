package impl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strings"

	"appforge/common/storage/internal"

	"github.com/tencentyun/cos-go-sdk-v5"
)

type cosUploader struct {
	market     string
	secretID   string
	secretKey  string
	bucketName string
	bucketUrl  string
}

func NewTencentUploader(market, secretID, secretKey, bucketName, bucketUrl string) (*cosUploader, error) {
	if market == "" || secretID == "" || secretKey == "" || bucketName == "" {
		return nil, fmt.Errorf("tencent uploader missing required parameters")
	}
	return &cosUploader{
		market:     market,
		secretID:   secretID,
		secretKey:  secretKey,
		bucketName: bucketName,
		bucketUrl:  bucketUrl,
	}, nil
}

func (u *cosUploader) Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	key := internal.GenerateObjectKey(header.Filename)
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	bucketUrl := strings.TrimSpace(u.bucketUrl)
	if bucketUrl == "" {
		bucketUrl = fmt.Sprintf("https://%s.cos.%s.myqcloud.com", u.bucketName, u.market)
	}

	uUrl, err := url.Parse(bucketUrl)
	if err != nil {
		return "", err
	}

	client := cos.NewClient(&cos.BaseURL{BucketURL: uUrl}, nil)

	// Ensure bucket exists and is public-read.
	exists, err := client.Bucket.IsExist(ctx)
	if err != nil {
		return "", err
	}
	if !exists {
		_, err = client.Bucket.Put(ctx, &cos.BucketPutOptions{XCosACL: "public-read"})
		if err != nil {
			return "", err
		}
	} else {
		_, err = client.Bucket.PutACL(ctx, &cos.BucketPutACLOptions{Header: &cos.ACLHeaderOptions{XCosACL: "public-read"}})
		if err != nil {
			return "", err
		}
	}

	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: contentType,
		},
	}
	_, err = client.Object.Put(ctx, key, bytes.NewReader(data), opt)
	if err != nil {
		return "", err
	}

	return "/" + key, nil
}
