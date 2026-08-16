// Code scaffolded by goctl. Safe to edit.

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"appforge/common/rpcauth"
	"appforge/common/siem"

	"github.com/zeromicro/go-zero/core/stores/cache"
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
	CacheRedis     cache.CacheConf `json:"CacheRedis" yaml:"CacheRedis"`
	SigningSecrets struct {
		MasterKeyBase64 string
	} `json:"SigningSecrets" yaml:"SigningSecrets"`
	SourceOAuth         SourceOAuthConfig         `json:"SourceOAuth" yaml:"SourceOAuth"`
	SourceTriggerWorker SourceTriggerWorkerConfig `json:"SourceTriggerWorker" yaml:"SourceTriggerWorker"`
	Billing             BillingConfig             `json:"Billing" yaml:"Billing"`
	LocalAgentGateway   LocalAgentGatewayConfig   `json:"LocalAgentGateway" yaml:"LocalAgentGateway"`
	OfflineLicense      OfflineLicenseConfig      `json:"OfflineLicense" yaml:"OfflineLicense"`
	Deployment          DeploymentConfig          `json:"Deployment" yaml:"Deployment"`
}

type DeploymentConfig struct {
	DeploymentId         string `json:"DeploymentId" yaml:"DeploymentId"`
	DeploymentMode       string `json:"DeploymentMode" yaml:"DeploymentMode"`
	ProductVersion       string `json:"ProductVersion" yaml:"ProductVersion"`
	TargetSchemaVersion  string `json:"TargetSchemaVersion" yaml:"TargetSchemaVersion"`
	MaxVersionSkew       int32  `json:"MaxVersionSkew" yaml:"MaxVersionSkew"`
	AgentProtocolCurrent int32  `json:"AgentProtocolCurrent" yaml:"AgentProtocolCurrent"`
	AgentProtocolMinimum int32  `json:"AgentProtocolMinimum" yaml:"AgentProtocolMinimum"`
}

// ApplyDeploymentEnvironment applies immutable release/deployment metadata
// supplied by Compose or Helm. It deliberately accepts no command or file path.
func ApplyDeploymentEnvironment(c *Config) error {
	if c == nil {
		return fmt.Errorf("config is required")
	}
	overrideString(&c.Deployment.DeploymentId, "APPFORGE_DEPLOYMENT_ID")
	overrideString(&c.Deployment.DeploymentMode, "APPFORGE_DEPLOYMENT_MODE")
	overrideString(&c.Deployment.ProductVersion, "APPFORGE_VERSION")
	overrideString(&c.Deployment.TargetSchemaVersion, "APPFORGE_SCHEMA_VERSION")
	if password := os.Getenv("APPFORGE_REDIS_PASSWORD"); password != "" {
		if len(c.CacheRedis) == 0 {
			return fmt.Errorf("CacheRedis is required when APPFORGE_REDIS_PASSWORD is set")
		}
		for index := range c.CacheRedis {
			c.CacheRedis[index].Pass = password
		}
	}
	if value := strings.TrimSpace(os.Getenv("APPFORGE_MAX_VERSION_SKEW")); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return fmt.Errorf("APPFORGE_MAX_VERSION_SKEW must be an integer: %w", err)
		}
		c.Deployment.MaxVersionSkew = int32(parsed)
	}
	if c.Deployment.DeploymentId == "" {
		c.Deployment.DeploymentId = "local-development"
	}
	if c.Deployment.DeploymentMode == "" {
		c.Deployment.DeploymentMode = "saas"
	}
	if c.Deployment.ProductVersion == "" {
		c.Deployment.ProductVersion = "0.0.0-dev"
	}
	if c.Deployment.TargetSchemaVersion == "" {
		c.Deployment.TargetSchemaVersion = "20260815_113_v7_air_gapped"
	}
	if c.Deployment.MaxVersionSkew <= 0 {
		c.Deployment.MaxVersionSkew = 1
	}
	if c.Deployment.AgentProtocolCurrent <= 0 {
		c.Deployment.AgentProtocolCurrent = 3
	}
	if c.Deployment.AgentProtocolMinimum <= 0 {
		c.Deployment.AgentProtocolMinimum = 2
	}
	if c.Deployment.AgentProtocolMinimum > c.Deployment.AgentProtocolCurrent {
		return fmt.Errorf("minimum Agent protocol cannot exceed current protocol")
	}
	switch c.Deployment.DeploymentMode {
	case "saas", "dedicated", "private", "hybrid":
	default:
		return fmt.Errorf("unsupported deployment mode %q", c.Deployment.DeploymentMode)
	}
	return nil
}

func overrideString(target *string, environmentKey string) {
	if value := strings.TrimSpace(os.Getenv(environmentKey)); value != "" {
		*target = value
	} else {
		*target = strings.TrimSpace(*target)
	}
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
