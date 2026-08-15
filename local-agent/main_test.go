package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNextAuthIsStrictlyMonotonicUnderConcurrency(t *testing.T) {
	dir := t.TempDir()
	current := state{AgentID: 42}
	const calls = 128
	timestamps := make(chan int64, calls)
	var workers sync.WaitGroup
	for i := 0; i < calls; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			auth := nextAuth(&current, dir)
			timestamps <- auth["timestamp"].(int64)
		}()
	}
	workers.Wait()
	close(timestamps)

	seen := make(map[int64]struct{}, calls)
	for timestamp := range timestamps {
		if _, exists := seen[timestamp]; exists {
			t.Fatalf("duplicate authentication timestamp %d", timestamp)
		}
		seen[timestamp] = struct{}{}
	}
	if len(seen) != calls {
		t.Fatalf("got %d unique timestamps, want %d", len(seen), calls)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted state
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.LastTimestamp != current.LastTimestamp {
		t.Fatalf("persisted timestamp %d differs from memory %d", persisted.LastTimestamp, current.LastTimestamp)
	}
}

func TestTaskJSONPreservesVersionID(t *testing.T) {
	raw := []byte(`{"id":7001,"tenant_id":1001,"app_id":2001,"version_id":3001,"builder_attempt":2}`)
	var item task
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	if item.VersionID != 3001 {
		t.Fatalf("version ID = %d, want 3001", item.VersionID)
	}
	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["version_id"] != float64(3001) {
		t.Fatalf("serialized version_id = %#v", fields["version_id"])
	}
}

func TestCertificateExpiresWithin(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "agent"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(2 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "client.crt")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if due, err := certificateExpiresWithin(path, 3*time.Hour, now); err != nil || !due {
		t.Fatalf("certificate should be due: due=%v err=%v", due, err)
	}
	if due, err := certificateExpiresWithin(path, time.Hour, now); err != nil || due {
		t.Fatalf("certificate should not be due: due=%v err=%v", due, err)
	}
}

func TestHealthCommandValidatesPrivateMTLSState(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(17), Subject: pkix.Name{CommonName: "agent-health"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	certificatePath := filepath.Join(root, "client.crt")
	privateKeyPath := filepath.Join(root, "client.key")
	caPath := filepath.Join(root, "agent-ca.crt")
	for path, value := range map[string][]byte{
		certificatePath: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		privateKeyPath:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
		caPath:          pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
	} {
		if err := os.WriteFile(path, value, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	current := state{AgentID: 81, GatewayURL: "https://agent.example.com:9443", Certificate: certificatePath,
		PrivateKey: privateKeyPath, ClientCA: caPath, GatewayCA: caPath, Protocol: protocolVersion, AgentVersion: version}
	raw, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "state.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := healthCommand([]string{"--state-dir", root}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(privateKeyPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := healthCommand([]string{"--state-dir", root}); err == nil {
		t.Fatal("health accepted a group/world-readable private key")
	}
}

func TestReadPrivateRegistrationToken(t *testing.T) {
	root := t.TempDir()
	tokenPath := filepath.Join(root, "registration.token")
	if err := os.WriteFile(tokenPath, []byte("one-time-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if token, err := readPrivateToken(tokenPath); err != nil || token != "one-time-token" {
		t.Fatalf("token=%q err=%v", token, err)
	}
	if err := os.Chmod(tokenPath, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := readPrivateToken(tokenPath); err == nil {
		t.Fatal("group-readable registration token was accepted")
	}
}

func TestSecretImportCommandUsesStrictPrivateFile(t *testing.T) {
	root := t.TempDir()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte(`{"keystorePassword":"store","keyPassword":"key"}`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	previousStdin := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = previousStdin
		_ = reader.Close()
	})
	if err := secretImportCommand([]string{"--secret-root", root, "--name", "release.json", "--input-stdin"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "release.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("Secret mode = %o", info.Mode().Perm())
	}
	secret, err := resolveLocalSigningSecret(root, "local-file:///release.json")
	if err != nil {
		t.Fatal(err)
	}
	secret.erase()
	if err := secretImportCommand([]string{"--secret-root", root, "--name", "../escape.json", "--input-stdin"}); err == nil {
		t.Fatal("Secret import accepted a path traversal name")
	}
}

func TestNewAgentCSRAndSafeSuffix(t *testing.T) {
	key, raw, err := newAgentCSR()
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		t.Fatal("CSR PEM was not generated")
	}
	request, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := request.CheckSignature(); err != nil {
		t.Fatal(err)
	}
	publicKey, ok := request.PublicKey.(*ecdsa.PublicKey)
	if !ok || publicKey.X.Cmp(key.PublicKey.X) != 0 {
		t.Fatal("CSR does not contain the generated public key")
	}
	if got := safeFileSuffix("ab/../CD:01"); got != "abCD01" {
		t.Fatalf("safe suffix = %q", got)
	}
}

func TestResolveLocalSigningSecret(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "signing.json")
	if err := os.WriteFile(path, []byte(`{"keystorePassword":"store-pass","keyPassword":"key-pass"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	secret, err := resolveLocalSigningSecret(root, "local-file:///signing.json")
	if err != nil {
		t.Fatal(err)
	}
	if secret.KeystorePassword != "store-pass" || secret.KeyPassword != "key-pass" {
		t.Fatalf("unexpected signing Secret: %#v", secret)
	}
	secret.erase()
	if secret.KeystorePassword != "" || secret.KeyPassword != "" {
		t.Fatal("signing Secret was not erased")
	}
}

func TestResolveLocalSigningSecretRejectsUnsafeFiles(t *testing.T) {
	root := t.TempDir()
	tooOpen := filepath.Join(root, "open.json")
	if err := os.WriteFile(tooOpen, []byte(`{"keystorePassword":"a","keyPassword":"b"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveLocalSigningSecret(root, "local-file:///open.json"); err == nil {
		t.Fatal("group-readable Secret was accepted")
	}
	unknown := filepath.Join(root, "unknown.json")
	if err := os.WriteFile(unknown, []byte(`{"keystorePassword":"a","keyPassword":"b","token":"forbidden"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveLocalSigningSecret(root, "local-file:///unknown.json"); err == nil {
		t.Fatal("unknown Secret field was accepted")
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte(`{"keystorePassword":"a","keyPassword":"b"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveLocalSigningSecret(root, "local-file:///linked.json"); err == nil {
		t.Fatal("symlinked Secret was accepted")
	}
}
