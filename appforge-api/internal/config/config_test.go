package config

import (
	"os"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
	"go.yaml.in/yaml/v3"
)

func TestAdminConfigLoadsAuditRoutes(t *testing.T) {
	data, err := os.ReadFile("../../etc/admin.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Audit AuditConfig `yaml:"Audit"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Audit.Routes) == 0 {
		t.Fatal("Audit.Routes must not be empty")
	}
	first := cfg.Audit.Routes[0]
	if first.Method == "" || first.Path == "" {
		t.Fatalf("invalid first audit route: %+v", first)
	}
}

func TestEtcdAdminConfigIncludesAllSIEMFields(t *testing.T) {
	canonicalData, err := os.ReadFile("../../etc/admin.yaml")
	if err != nil {
		t.Fatal(err)
	}
	etcdData, err := os.ReadFile("../../../deploy/etcd/admin-api.yaml")
	if err != nil {
		t.Fatal(err)
	}
	type auditEnvelope struct {
		Audit struct {
			SIEM map[string]any `yaml:"SIEM"`
		} `yaml:"Audit"`
	}
	var canonical auditEnvelope
	if err := yaml.Unmarshal(canonicalData, &canonical); err != nil {
		t.Fatal(err)
	}
	var etcd auditEnvelope
	if err := yaml.Unmarshal(etcdData, &etcd); err != nil {
		t.Fatal(err)
	}
	for field := range canonical.Audit.SIEM {
		if _, exists := etcd.Audit.SIEM[field]; !exists {
			t.Errorf("deploy/etcd/admin-api.yaml Audit.SIEM is missing %s", field)
		}
	}
}

func TestEtcdMergedAdminConfigLoadsStrictly(t *testing.T) {
	commonData, err := os.ReadFile("../../../deploy/etcd/common.yaml")
	if err != nil {
		t.Fatal(err)
	}
	adminData, err := os.ReadFile("../../../deploy/etcd/admin-api.yaml")
	if err != nil {
		t.Fatal(err)
	}
	merged := make(map[string]any)
	if err := yaml.Unmarshal(commonData, &merged); err != nil {
		t.Fatal(err)
	}
	service := make(map[string]any)
	if err := yaml.Unmarshal(adminData, &service); err != nil {
		t.Fatal(err)
	}
	mergeConfigMap(merged, service)
	data, err := yaml.Marshal(merged)
	if err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := conf.LoadFromYamlBytes(data, &cfg); err != nil {
		t.Fatalf("strict etcd admin config load failed: %v", err)
	}
}

func mergeConfigMap(destination, source map[string]any) {
	for key, value := range source {
		sourceMap, sourceIsMap := value.(map[string]any)
		destinationMap, destinationIsMap := destination[key].(map[string]any)
		if sourceIsMap && destinationIsMap {
			mergeConfigMap(destinationMap, sourceMap)
			continue
		}
		destination[key] = value
	}
}

func TestApplySIEMEnvironmentSupportsRFC5424TLS(t *testing.T) {
	t.Setenv("APPFORGE_SIEM_ENDPOINT", "syslog+tls://siem.customer.test:6514")
	t.Setenv("APPFORGE_SIEM_TOKEN_FILE", "/etc/appforge/siem/token")
	t.Setenv("APPFORGE_SIEM_CA_FILE", "/etc/appforge/siem/ca.crt")
	t.Setenv("APPFORGE_SIEM_SYSLOG_HOSTNAME", "api-0")
	t.Setenv("APPFORGE_SIEM_SYSLOG_APP_NAME", "appforge-admin")
	var cfg Config
	ApplySIEMEnvironment(&cfg)
	if !cfg.Audit.SIEM.Enabled || cfg.Audit.SIEM.Endpoint != "syslog+tls://siem.customer.test:6514" {
		t.Fatalf("SIEM environment was not applied: %+v", cfg.Audit.SIEM)
	}
	if cfg.Audit.SIEM.CACertificate != "/etc/appforge/siem/ca.crt" ||
		cfg.Audit.SIEM.SyslogHostname != "api-0" || cfg.Audit.SIEM.SyslogAppName != "appforge-admin" {
		t.Fatalf("SIEM TLS metadata was not applied: %+v", cfg.Audit.SIEM)
	}
}
