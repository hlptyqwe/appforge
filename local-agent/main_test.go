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
