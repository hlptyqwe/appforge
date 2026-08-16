package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestExecutorBundleOmitsControlPlaneSigningSecretReference(t *testing.T) {
	bundle := buildManifest{SigningSecretRef: ""}
	encoded, err := json.Marshal(&bundle)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "signing_secret_ref") {
		t.Fatalf("control-plane Secret reference field leaked into strict executor bundle: %s", encoded)
	}
}

func TestDownloadArtifactInputsVerifiesBytesAndUsesPrivateFiles(t *testing.T) {
	payload := []byte("private-keystore-fixture")
	digest := digestBytes(payload)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/artifacts/download/token" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(bytes.NewReader(payload)),
			ContentLength: int64(len(payload)), Header: http.Header{"X-AppForge-Sha256": []string{digest}}}, nil
	})}
	bundle := &buildManifest{Inputs: []buildInput{{Role: "keystore", ObjectID: 9, OriginalName: "release.jks",
		SizeBytes: int64(len(payload)), SHA256: digest, DownloadPath: "/v1/artifacts/download/token"}}}
	workDir := t.TempDir()
	if err := downloadArtifactInputs(context.Background(), client, "https://gateway.example:9443", workDir, bundle); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(bundle.Inputs[0].LocalPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("download mode=%o", info.Mode().Perm())
	}
	actual, err := os.ReadFile(bundle.Inputs[0].LocalPath)
	if err != nil || !bytes.Equal(actual, payload) {
		t.Fatalf("downloaded bytes=%q err=%v", actual, err)
	}

	bundle.Inputs[0].SHA256 = digestBytes([]byte("tampered"))
	if err := downloadArtifactInputs(context.Background(), client, "https://gateway.example:9443", workDir, bundle); err == nil {
		t.Fatal("tampered input digest was accepted")
	}
}

func TestUploadArtifactOutputHashesPrivateTaskFile(t *testing.T) {
	payload := []byte("signed-apk-fixture")
	digest := digestBytes(payload)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/artifacts/upload/token" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-AppForge-Sha256") != digest || r.ContentLength != int64(len(payload)) {
			t.Fatalf("unexpected integrity headers")
		}
		actual, _ := io.ReadAll(r.Body)
		if !bytes.Equal(actual, payload) {
			t.Fatalf("uploaded bytes=%q", actual)
		}
		encoded, _ := json.Marshal(artifactUploadResponse{Reference: "storage-object://91", SHA256: digest, SizeBytes: int64(len(payload))})
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(bytes.NewReader(encoded)), Header: make(http.Header)}, nil
	})}
	workDir := t.TempDir()
	artifact := filepath.Join(workDir, "channel.apk")
	if err := os.WriteFile(artifact, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	bundle := &buildManifest{Outputs: []buildOutput{{Role: "built_apk", UploadPath: "/v1/artifacts/upload/token"}}}
	result, err := uploadArtifactOutput(context.Background(), client, "https://gateway.example:9443", workDir, bundle, "built_apk", artifact)
	if err != nil {
		t.Fatal(err)
	}
	if result.Reference != "storage-object://91" || result.SHA256 != digest || result.SizeBytes != int64(len(payload)) {
		t.Fatalf("unexpected upload result: %#v", result)
	}
	outside := filepath.Join(t.TempDir(), "outside.apk")
	if err := os.WriteFile(outside, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := uploadArtifactOutput(context.Background(), client, "https://gateway.example:9443", workDir, bundle, "built_apk", outside); err == nil {
		t.Fatal("output path outside task directory was accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestGatewayArtifactURLRejectsHostAndQueryInjection(t *testing.T) {
	for _, value := range []string{"https://evil.example/v1/artifacts/download/token", "/v1/artifacts/download/token?secret=1", "/v1/artifacts/download/a/b"} {
		if _, err := gatewayArtifactURL("https://gateway.example:9443", value, "/v1/artifacts/download/"); err == nil {
			t.Fatalf("unsafe transfer path %q was accepted", value)
		}
	}
}

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

func TestAuthenticatedRequestsArriveInTimestampOrder(t *testing.T) {
	dir := t.TempDir()
	current := state{AgentID: 42}
	var mu sync.Mutex
	var arrived []int64
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload struct {
			Auth struct {
				Timestamp int64 `json:"timestamp"`
			} `json:"auth"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			return nil, err
		}
		mu.Lock()
		arrived = append(arrived, payload.Auth.Timestamp)
		mu.Unlock()
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(bytes.NewReader([]byte(`{}`))), Header: make(http.Header)}, nil
	})}
	const calls = 32
	var workers sync.WaitGroup
	for index := 0; index < calls; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := postAuthenticatedJSON(context.Background(), client, &current, dir, "https://gateway.example/v1/heartbeat", map[string]any{}, nil); err != nil {
				t.Errorf("authenticated request: %v", err)
			}
		}()
	}
	workers.Wait()
	if len(arrived) != calls {
		t.Fatalf("arrived=%d want=%d", len(arrived), calls)
	}
	for index := 1; index < len(arrived); index++ {
		if arrived[index] <= arrived[index-1] {
			t.Fatalf("timestamps arrived out of order at %d: %v", index, arrived)
		}
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
