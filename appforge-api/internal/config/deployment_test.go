package config

import (
	"testing"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func TestApplyDeploymentEnvironmentDefaultsAndOverrides(t *testing.T) {
	for _, key := range []string{
		"APPFORGE_DEPLOYMENT_ID", "APPFORGE_DEPLOYMENT_MODE", "APPFORGE_VERSION",
		"APPFORGE_SCHEMA_VERSION", "APPFORGE_MAX_VERSION_SKEW", "APPFORGE_REDIS_PASSWORD",
	} {
		t.Setenv(key, "")
	}
	var defaults Config
	if err := ApplyDeploymentEnvironment(&defaults); err != nil {
		t.Fatal(err)
	}
	if defaults.Deployment.DeploymentMode != "saas" || defaults.Deployment.AgentProtocolCurrent != 3 || defaults.Deployment.AgentProtocolMinimum != 2 {
		t.Fatalf("unexpected defaults: %+v", defaults.Deployment)
	}

	t.Setenv("APPFORGE_DEPLOYMENT_ID", "customer-a")
	t.Setenv("APPFORGE_DEPLOYMENT_MODE", "private")
	t.Setenv("APPFORGE_VERSION", "1.2.3")
	t.Setenv("APPFORGE_SCHEMA_VERSION", "schema-123")
	t.Setenv("APPFORGE_MAX_VERSION_SKEW", "2")
	var overridden Config
	if err := ApplyDeploymentEnvironment(&overridden); err != nil {
		t.Fatal(err)
	}
	if overridden.Deployment.DeploymentId != "customer-a" || overridden.Deployment.ProductVersion != "1.2.3" || overridden.Deployment.MaxVersionSkew != 2 {
		t.Fatalf("environment was not applied: %+v", overridden.Deployment)
	}
}

func TestApplyDeploymentEnvironmentOverridesArtifactTicketRedisPassword(t *testing.T) {
	t.Setenv("APPFORGE_REDIS_PASSWORD", "redis-secret-with:/symbols")
	var configured Config
	configured.CacheRedis = append(configured.CacheRedis, cache.NodeConf{RedisConf: redis.RedisConf{Host: "redis:6379", Type: "node"}})
	if err := ApplyDeploymentEnvironment(&configured); err != nil {
		t.Fatal(err)
	}
	if configured.CacheRedis[0].Pass != "redis-secret-with:/symbols" {
		t.Fatal("Redis password environment override was not applied")
	}
	var missing Config
	if err := ApplyDeploymentEnvironment(&missing); err == nil {
		t.Fatal("Redis password without CacheRedis configuration was accepted")
	}
}

func TestApplyDeploymentEnvironmentRejectsInvalidMetadata(t *testing.T) {
	t.Setenv("APPFORGE_DEPLOYMENT_MODE", "unsupported")
	var config Config
	if err := ApplyDeploymentEnvironment(&config); err == nil {
		t.Fatal("unsupported deployment mode must fail")
	}
}
