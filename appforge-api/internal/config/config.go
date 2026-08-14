// Code scaffolded by goctl. Safe to edit.

package config

import (
	"time"

	"appforge/common/rpcauth"
	"appforge/common/siem"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Jwt struct {
		AccessSecret string
		AccessExpire int64
	} `json:"Jwt" yaml:"Jwt"`
	Audit          AuditConfig
	InternalRpc    rpcauth.Config
	SystemRpc      zrpc.RpcClientConf
	CoreRpc        zrpc.RpcClientConf
	BuilderRpc     zrpc.RpcClientConf
	SigningSecrets struct {
		MasterKeyBase64 string
	} `json:"SigningSecrets" yaml:"SigningSecrets"`
	SourceOAuth         SourceOAuthConfig         `json:"SourceOAuth" yaml:"SourceOAuth"`
	SourceTriggerWorker SourceTriggerWorkerConfig `json:"SourceTriggerWorker" yaml:"SourceTriggerWorker"`
	Billing             BillingConfig             `json:"Billing" yaml:"Billing"`
	LocalAgentGateway   LocalAgentGatewayConfig   `json:"LocalAgentGateway" yaml:"LocalAgentGateway"`
	OfflineLicense      OfflineLicenseConfig      `json:"OfflineLicense" yaml:"OfflineLicense"`
}

type OfflineLicenseConfig struct {
	Enabled                bool          `json:"Enabled" yaml:"Enabled"`
	LicenseFile            string        `json:"LicenseFile" yaml:"LicenseFile"`
	PublicKeyFile          string        `json:"PublicKeyFile" yaml:"PublicKeyFile"`
	StateFile              string        `json:"StateFile" yaml:"StateFile"`
	DeploymentId           string        `json:"DeploymentId" yaml:"DeploymentId"`
	DeploymentMode         string        `json:"DeploymentMode" yaml:"DeploymentMode"`
	ClockRollbackTolerance time.Duration `json:"ClockRollbackTolerance" yaml:"ClockRollbackTolerance"`
}

type LocalAgentGatewayConfig struct {
	Enabled             bool   `json:"Enabled" yaml:"Enabled"`
	ListenOn            string `json:"ListenOn" yaml:"ListenOn"`
	ServerCertificate   string `json:"ServerCertificate" yaml:"ServerCertificate"`
	ServerPrivateKey    string `json:"ServerPrivateKey" yaml:"ServerPrivateKey"`
	ClientCACertificate string `json:"ClientCACertificate" yaml:"ClientCACertificate"`
}

type BillingConfig struct {
	StripeSecretKey     string `json:"StripeSecretKey" yaml:"StripeSecretKey"`
	StripeWebhookSecret string `json:"StripeWebhookSecret" yaml:"StripeWebhookSecret"`
	StripeAPIBaseURL    string `json:"StripeAPIBaseURL" yaml:"StripeAPIBaseURL"`
	SuccessURL          string `json:"SuccessURL" yaml:"SuccessURL"`
	CancelURL           string `json:"CancelURL" yaml:"CancelURL"`
}

type SourceOAuthConfig struct {
	SuccessRedirect string                    `json:"SuccessRedirect" yaml:"SuccessRedirect"`
	WebhookBaseURL  string                    `json:"WebhookBaseURL" yaml:"WebhookBaseURL"`
	GitHub          SourceOAuthProviderConfig `json:"GitHub" yaml:"GitHub"`
	GitLab          SourceOAuthProviderConfig `json:"GitLab" yaml:"GitLab"`
}

type SourceTriggerWorkerConfig struct {
	Enabled      bool          `json:"Enabled" yaml:"Enabled"`
	WorkerId     string        `json:"WorkerId" yaml:"WorkerId"`
	PollInterval time.Duration `json:"PollInterval" yaml:"PollInterval"`
	LeaseSeconds int32         `json:"LeaseSeconds" yaml:"LeaseSeconds"`
}

type SourceOAuthProviderConfig struct {
	ClientId     string `json:"ClientId" yaml:"ClientId"`
	ClientSecret string `json:"ClientSecret" yaml:"ClientSecret"`
	AuthorizeURL string `json:"AuthorizeURL" yaml:"AuthorizeURL"`
	TokenURL     string `json:"TokenURL" yaml:"TokenURL"`
	ApiBaseURL   string `json:"ApiBaseURL" yaml:"ApiBaseURL"`
	RedirectURL  string `json:"RedirectURL" yaml:"RedirectURL"`
}

type AuditConfig struct {
	Routes []AuditRoute `json:"Routes" yaml:"Routes"`
	SIEM   siem.Config  `json:"SIEM" yaml:"SIEM"`
}

type AuditRoute struct {
	Method string `json:"Method" yaml:"Method"`
	Path   string `json:"Path" yaml:"Path"`
}
