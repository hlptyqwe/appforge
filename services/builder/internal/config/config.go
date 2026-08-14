// Code scaffolded for the APK Builder RPC.
package config

import (
	"time"

	"appforge/common/rpcauth"
	"appforge/common/storage"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	CoreRpc    zrpc.RpcClientConf `json:"CoreRpc" yaml:"CoreRpc"`
	BuilderRpc zrpc.RpcClientConf `json:"BuilderRpc" yaml:"BuilderRpc"`
	Builder    struct {
		Id           string
		LeaseSeconds int32
		PollInterval time.Duration
		TempDir      string
	} `json:"Builder" yaml:"Builder"`
	ObjectCleanup struct {
		Interval   time.Duration
		StaleAfter time.Duration
		BatchSize  int32
	} `json:"ObjectCleanup" yaml:"ObjectCleanup"`
	ObjectStorage  ObjectStorageConfig `json:"ObjectStorage" yaml:"ObjectStorage"`
	SigningSecrets struct {
		MasterKeyBase64 string
	} `json:"SigningSecrets" yaml:"SigningSecrets"`
	InternalRpc rpcauth.Config
}

type ObjectStorageConfig struct {
	OssType int64
	Minio   struct {
		Endpoint        string
		BucketUrl       string
		AccessKeyId     string
		AccessKeySecret string
		BucketName      string
	}
}

func (c ObjectStorageConfig) StorageConfig() storage.Config {
	return storage.Config{
		OssType: storage.OssType(c.OssType),
		Minio: &storage.MinioConfig{
			Endpoint: c.Minio.Endpoint, BucketUrl: c.Minio.BucketUrl,
			AccessKeyId: c.Minio.AccessKeyId, AccessKeySecret: c.Minio.AccessKeySecret,
			BucketName: c.Minio.BucketName,
		},
	}
}
