package etcd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClientConfigSupportsAuthentication(t *testing.T) {
	t.Setenv("APPFORGE_ETCD_USERNAME", "appforge")
	t.Setenv("APPFORGE_ETCD_PASSWORD", "secret")
	cfg, err := clientConfig([]string{"https://etcd.example:2379"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Username != "appforge" || cfg.Password != "secret" {
		t.Fatal("credentials were not propagated")
	}
}

func TestClientConfigRejectsPartialMTLS(t *testing.T) {
	t.Setenv("APPFORGE_ETCD_CERT_FILE", "/tmp/client.crt")
	if _, err := clientConfig([]string{"https://etcd.example:2379"}); err == nil {
		t.Fatal("expected partial mTLS configuration to fail")
	}
}

func TestClientConfigRejectsInvalidCA(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ca.crt")
	if err := os.WriteFile(path, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPFORGE_ETCD_CA_FILE", path)
	if _, err := clientConfig([]string{"https://etcd.example:2379"}); err == nil {
		t.Fatal("expected invalid CA to fail")
	}
}
