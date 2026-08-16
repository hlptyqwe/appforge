package config

import (
	"os"
	"testing"

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
