// Code scaffolded for the APK distribution core RPC.
package config

import (
	"time"

	"appforge/common/rpcauth"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Mysql struct {
		DataSource string
	} `json:"Mysql" yaml:"Mysql"`
	CacheRedis     cache.CacheConf `json:"CacheRedis" yaml:"CacheRedis"`
	SigningSecrets struct {
		MasterKeyBase64 string
	} `json:"SigningSecrets" yaml:"SigningSecrets"`
	WebhookWorker struct {
		Enabled      bool
		PollInterval time.Duration
		HttpTimeout  time.Duration
		BatchSize    int32
	} `json:"WebhookWorker" yaml:"WebhookWorker"`
	BillingWorker struct {
		Enabled      bool
		PollInterval time.Duration
	} `json:"BillingWorker" yaml:"BillingWorker"`
	EnterpriseWorker struct {
		Enabled      bool
		PollInterval time.Duration
		OfflineAfter time.Duration
	} `json:"EnterpriseWorker" yaml:"EnterpriseWorker"`
	AgentPKI struct {
		CACertificateFile string
		CAPrivateKeyFile  string
		CertificateTTL    time.Duration
	} `json:"AgentPKI" yaml:"AgentPKI"`
	InternalRpc rpcauth.Config
}
