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
