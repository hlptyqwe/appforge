package offlinelicense

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSignedLicenseVerificationAndPersistentRollbackProtection(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	dir := t.TempDir()
	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPath := filepath.Join(dir, "vendor-public.pem")
	if err := os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	payload := Payload{LicenseID: "lic-001", Customer: "ACME", DeploymentID: "prod-hk-1",
		DeploymentModes: []string{"private", "hybrid"}, Features: []string{"local-agent"},
		NotBefore: now.Add(-time.Hour).UnixMilli(), NotAfter: now.Add(24 * time.Hour).UnixMilli(),
		IssuedAt: now.Add(-time.Hour).UnixMilli(), Sequence: 2, MaxTenants: 10, MaxBuilders: 4}
	envelope, err := Sign(payload, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	licensePath := filepath.Join(dir, "license.json")
	raw, _ := json.MarshalIndent(envelope, "", "  ")
	if err := os.WriteFile(licensePath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	config := Config{LicenseFile: licensePath, PublicKeyFile: publicPath, StateFile: filepath.Join(dir, "state", "clock.json"),
		DeploymentID: "prod-hk-1", DeploymentMode: "private", ClockRollbackTolerance: time.Minute}
	verified, err := VerifyFile(config, now)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Payload.Customer != "ACME" || len(verified.Fingerprint) != 64 {
		t.Fatalf("unexpected verified license: %#v", verified)
	}
	if _, err := VerifyFile(config, now.Add(-2*time.Minute)); err == nil || !strings.Contains(err.Error(), "clock rollback") {
		t.Fatalf("expected clock rollback rejection, got %v", err)
	}
}

func TestLicenseRejectsTamperingBindingExpiryAndSequenceRollback(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().UTC().Truncate(time.Second)
	dir := t.TempDir()
	publicDER, _ := x509.MarshalPKIXPublicKey(publicKey)
	publicPath := filepath.Join(dir, "public.pem")
	_ = os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}), 0o600)
	base := Payload{LicenseID: "lic-2", Customer: "Customer", DeploymentID: "deployment-a", DeploymentModes: []string{"private"},
		NotBefore: now.Add(-time.Hour).UnixMilli(), NotAfter: now.Add(time.Hour).UnixMilli(), IssuedAt: now.Add(-time.Hour).UnixMilli(), Sequence: 3}
	write := func(name string, payload Payload, tamper bool) string {
		envelope, err := Sign(payload, privateKey)
		if err != nil {
			t.Fatal(err)
		}
		if tamper {
			envelope.Payload.Customer = "Attacker"
		}
		raw, _ := json.Marshal(envelope)
		path := filepath.Join(dir, name)
		_ = os.WriteFile(path, raw, 0o600)
		return path
	}
	config := Config{PublicKeyFile: publicPath, StateFile: filepath.Join(dir, "state.json"), DeploymentID: "deployment-a", DeploymentMode: "private"}
	config.LicenseFile = write("valid.json", base, false)
	if _, err := VerifyFile(config, now); err != nil {
		t.Fatal(err)
	}
	config.LicenseFile = write("tampered.json", base, true)
	if _, err := VerifyFile(config, now); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("expected signature rejection, got %v", err)
	}
	config.LicenseFile = write("old.json", func() Payload { item := base; item.Sequence = 2; return item }(), false)
	if _, err := VerifyFile(config, now); err == nil || !strings.Contains(err.Error(), "license rollback") {
		t.Fatalf("expected sequence rollback rejection, got %v", err)
	}
	config.LicenseFile = write("expired.json", func() Payload {
		item := base
		item.NotAfter = now.Add(-time.Minute).UnixMilli()
		item.NotBefore = now.Add(-2 * time.Hour).UnixMilli()
		item.IssuedAt = item.NotBefore
		item.Sequence = 4
		return item
	}(), false)
	if _, err := VerifyFile(config, now); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expiry rejection, got %v", err)
	}
	config.LicenseFile = write("binding.json", func() Payload { item := base; item.Sequence = 4; return item }(), false)
	config.DeploymentID = "deployment-b"
	if _, err := VerifyFile(config, now); err == nil || !strings.Contains(err.Error(), "different deployment") {
		t.Fatalf("expected deployment binding rejection, got %v", err)
	}
}
