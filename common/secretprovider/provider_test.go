package secretprovider

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalFileProviderResolvesStrictSigningSecret(t *testing.T) {
	root := t.TempDir()
	secretPath := filepath.Join(root, "signing.json")
	if err := os.WriteFile(secretPath, []byte(`{"keystorePassword":"store","keyPassword":"key"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	provider, err := NewLocalFileProvider(root)
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := New(0, provider)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := resolver.ResolveSigningSecret(context.Background(), "local-file:///signing.json")
	if err != nil {
		t.Fatal(err)
	}
	if secret.KeystorePassword != "store" || secret.KeyPassword != "key" {
		t.Fatal("resolved secret differs")
	}
	secret.Erase()
	if secret.KeystorePassword != "" || secret.KeyPassword != "" {
		t.Fatal("secret was not erased")
	}
}

func TestLocalFileProviderRejectsUnsafeModeAndSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	if err := os.WriteFile(target, []byte(`{"keystorePassword":"store","keyPassword":"key"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	provider, _ := NewLocalFileProvider(root)
	resolver, _ := New(0, provider)
	if _, err := resolver.Resolve(context.Background(), "local-file:///target.json"); err == nil {
		t.Fatal("expected unsafe file mode rejection")
	}
	if err := os.Chmod(target, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "link.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(context.Background(), "local-file:///link.json"); err == nil {
		t.Fatal("expected local symlink rejection")
	}
}

func TestVaultProviderResolvesKVv2WithoutLeakingToken(t *testing.T) {
	root := t.TempDir()
	tokenPath := filepath.Join(root, "token")
	if err := os.WriteFile(tokenPath, []byte("vault-test-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/secret/data/appforge/signing" || r.Header.Get("X-Vault-Token") != "vault-test-token" {
			return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("unauthorized")), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":{"data":{"keystorePassword":"vault-store","keyPassword":"vault-key"}}}`)), Header: make(http.Header)}, nil
	})}
	provider, err := NewVaultProvider("http://vault.test", tokenPath, "", client, true)
	if err != nil {
		t.Fatal(err)
	}
	resolver, _ := New(0, provider)
	secret, err := resolver.ResolveSigningSecret(context.Background(), "vault://secret/data/appforge/signing")
	if err != nil {
		t.Fatal(err)
	}
	if secret.KeystorePassword != "vault-store" || strings.Contains(secret.KeystorePassword, "token") {
		t.Fatal("Vault secret resolution failed")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestVaultProviderRequiresTLSByDefault(t *testing.T) {
	if _, err := NewVaultProvider("http://vault:8200", "/token", "", nil, false); err == nil {
		t.Fatal("expected cleartext Vault address rejection")
	}
}
