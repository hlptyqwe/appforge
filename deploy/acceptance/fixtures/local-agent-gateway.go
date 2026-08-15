package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"sync"
	"time"
)

type gateway struct {
	mu          sync.Mutex
	claims      int
	completed   bool
	events      *os.File
	lastAuth    int64
	nonceValues map[string]struct{}
	signingCA   *x509.Certificate
	signingKey  *ecdsa.PrivateKey
	caPEM       string
	revokeFile  string
}

func main() {
	address := flag.String("addr", "127.0.0.1:19443", "listen address")
	certificateFile := flag.String("cert", "", "server certificate")
	keyFile := flag.String("key", "", "server private key")
	clientCAFile := flag.String("client-ca", "", "client CA")
	signingCAFile := flag.String("signing-ca", "", "certificate rotation signing CA")
	signingKeyFile := flag.String("signing-key", "", "certificate rotation signing key")
	eventsFile := flag.String("events", "", "JSONL event output")
	revokeFile := flag.String("revoke-file", "", "when present, reject all Agent RPCs")
	flag.Parse()
	if *certificateFile == "" || *keyFile == "" || *clientCAFile == "" || *signingCAFile == "" ||
		*signingKeyFile == "" || *eventsFile == "" || *revokeFile == "" {
		log.Fatal("cert, key, client-ca, signing-ca, signing-key, events and revoke-file are required")
	}
	caPEM, err := os.ReadFile(*clientCAFile)
	if err != nil {
		log.Fatal(err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caPEM) {
		log.Fatal("client CA is invalid")
	}
	events, err := os.OpenFile(*eventsFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		log.Fatal(err)
	}
	defer events.Close()

	signingCA, signingKey, signingCAPEM, err := loadSigningCA(*signingCAFile, *signingKeyFile)
	if err != nil {
		log.Fatal(err)
	}
	g := &gateway{events: events, nonceValues: make(map[string]struct{}), signingCA: signingCA,
		signingKey: signingKey, caPEM: signingCAPEM, revokeFile: *revokeFile}
	server := &http.Server{
		Addr:              *address,
		Handler:           g,
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs: clientCAs},
	}
	log.Printf("synthetic Local Agent gateway listening on %s", *address)
	log.Fatal(server.ListenAndServeTLS(*certificateFile, *keyFile))
}

func (g *gateway) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet && request.URL.Path == "/health" {
		g.writeJSON(response, http.StatusOK, map[string]any{"status": "ok"})
		return
	}
	if request.Method != http.MethodPost {
		g.writeJSON(response, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	if _, err := os.Stat(g.revokeFile); err == nil {
		serial := ""
		if request.TLS != nil && len(request.TLS.PeerCertificates) == 1 {
			serial = request.TLS.PeerCertificates[0].SerialNumber.Text(16)
		}
		g.record(map[string]any{"path": "revoked_request_rejected", "request_path": request.URL.Path,
			"certificate_serial": serial})
		g.writeJSON(response, http.StatusForbidden, map[string]any{"error": "Local Agent is revoked"})
		return
	}
	defer request.Body.Close()
	var body map[string]any
	decoder := json.NewDecoder(io.LimitReader(request.Body, 1<<20))
	if err := decoder.Decode(&body); err != nil {
		g.writeJSON(response, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	if !g.acceptAuth(body) {
		g.writeJSON(response, http.StatusConflict, map[string]any{"error": "replayed authentication envelope"})
		return
	}
	attempt := int64(number(body["builder_attempt"]))
	g.record(map[string]any{"path": request.URL.Path, "builder_attempt": attempt, "auth": body["auth"]})

	switch request.URL.Path {
	case "/v1/heartbeat", "/v1/tasks/renew", "/v1/tasks/progress", "/v1/tasks/fail":
		g.writeJSON(response, http.StatusOK, map[string]any{"ok": true})
	case "/v1/claim":
		g.handleClaim(response)
	case "/v1/certificates/rotate":
		g.handleRotation(response, body)
	case "/v1/tasks/complete":
		if attempt != 2 {
			g.record(map[string]any{"path": "stale_attempt_rejected", "builder_attempt": attempt})
			g.writeJSON(response, http.StatusConflict, map[string]any{"error": "stale builder attempt"})
			return
		}
		if fmt.Sprint(body["apk_reference"]) == "" || fmt.Sprint(body["apk_sha256"]) == "" {
			g.writeJSON(response, http.StatusBadRequest, map[string]any{"error": "artifact metadata incomplete"})
			return
		}
		g.mu.Lock()
		g.completed = true
		g.mu.Unlock()
		g.record(map[string]any{"path": "attempt_completed", "builder_attempt": attempt,
			"apk_reference": body["apk_reference"], "apk_sha256": body["apk_sha256"]})
		g.writeJSON(response, http.StatusOK, map[string]any{"ok": true})
	default:
		g.writeJSON(response, http.StatusNotFound, map[string]any{"error": "unknown endpoint"})
	}
}

func (g *gateway) handleRotation(response http.ResponseWriter, body map[string]any) {
	csrPEM := fmt.Sprint(body["csr_pem"])
	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		g.writeJSON(response, http.StatusBadRequest, map[string]any{"error": "CSR is invalid"})
		return
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil || csr.CheckSignature() != nil {
		g.writeJSON(response, http.StatusBadRequest, map[string]any{"error": "CSR signature is invalid"})
		return
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		g.writeJSON(response, http.StatusInternalServerError, map[string]any{"error": "serial generation failed"})
		return
	}
	now := time.Now().UTC()
	template := &x509.Certificate{SerialNumber: serial, Subject: csr.Subject, NotBefore: now.Add(-time.Minute),
		NotAfter: now.Add(7 * 24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, g.signingCA, csr.PublicKey, g.signingKey)
	if err != nil {
		g.writeJSON(response, http.StatusInternalServerError, map[string]any{"error": "certificate signing failed"})
		return
	}
	certificatePEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	g.record(map[string]any{"path": "certificate_rotated", "certificate_serial": serial.Text(16),
		"certificate_not_after": template.NotAfter.UnixMilli()})
	g.writeJSON(response, http.StatusOK, map[string]any{
		"data": map[string]any{"id": 81},
		"certificate": map[string]any{"serial_number": serial.Text(16), "certificate_pem": certificatePEM,
			"not_after": template.NotAfter.UnixMilli()},
		"ca_certificate_pem": g.caPEM,
	})
}

func (g *gateway) handleClaim(response http.ResponseWriter) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.completed || g.claims >= 2 {
		g.writeJSON(response, http.StatusOK, map[string]any{"task": nil, "artifact_mode": 1, "bundle": nil})
		return
	}
	g.claims++
	attempt := g.claims
	task := map[string]any{"id": 7001, "tenant_id": 1001, "app_id": 2001, "version_id": 3001,
		"builder_attempt": attempt, "channel_code": "recovery-acceptance", "version_code": 1, "version_name": "1.0"}
	bundle := map[string]any{
		"schema_version": 3, "task": task, "package_name": "com.example.local", "api_host": "https://api.example.com",
		"channel_name": "Recovery Acceptance", "landing_url": "https://example.com", "key_alias": "release",
		"signing_secret_ref":        "local-file:///acceptance.json",
		"signer_certificate_sha256": "0000000000000000000000000000000000000000000000000000000000000000",
		"branding_snapshot_json":    "", "template_snapshot_json": "", "inputs": []any{}, "blocked_reason": "",
	}
	g.recordLocked(map[string]any{"path": "attempt_claimed", "builder_attempt": attempt})
	g.writeJSON(response, http.StatusOK, map[string]any{"task": task, "artifact_mode": 1, "bundle": bundle})
}

func (g *gateway) acceptAuth(body map[string]any) bool {
	auth, ok := body["auth"].(map[string]any)
	if !ok || int64(number(auth["agent_id"])) != 81 {
		return false
	}
	nonce := fmt.Sprint(auth["nonce"])
	timestamp := int64(number(auth["timestamp"]))
	if nonce == "" || timestamp <= 0 {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.nonceValues[nonce]; exists {
		return false
	}
	g.nonceValues[nonce] = struct{}{}
	if timestamp > g.lastAuth {
		g.lastAuth = timestamp
	}
	return true
}

func (g *gateway) record(event map[string]any) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.recordLocked(event)
}

func (g *gateway) recordLocked(event map[string]any) {
	event["recorded_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	encoded, _ := json.Marshal(event)
	_, _ = g.events.Write(append(encoded, '\n'))
	_ = g.events.Sync()
}

func (g *gateway) writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func number(value any) float64 {
	result, _ := value.(float64)
	return result
}

func loadSigningCA(certificateFile, keyFile string) (*x509.Certificate, *ecdsa.PrivateKey, string, error) {
	certificatePEM, err := os.ReadFile(certificateFile)
	if err != nil {
		return nil, nil, "", err
	}
	certificateBlock, _ := pem.Decode(certificatePEM)
	if certificateBlock == nil {
		return nil, nil, "", fmt.Errorf("signing CA certificate is invalid")
	}
	certificate, err := x509.ParseCertificate(certificateBlock.Bytes)
	if err != nil || !certificate.IsCA {
		return nil, nil, "", fmt.Errorf("signing CA certificate is not a CA")
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, nil, "", err
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, "", fmt.Errorf("signing CA key is invalid")
	}
	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		parsed, parseErr := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if parseErr != nil {
			return nil, nil, "", fmt.Errorf("parse signing CA key: %w", err)
		}
		var ok bool
		key, ok = parsed.(*ecdsa.PrivateKey)
		if !ok {
			return nil, nil, "", fmt.Errorf("signing CA key must be ECDSA")
		}
	}
	if !key.PublicKey.Equal(certificate.PublicKey) {
		return nil, nil, "", fmt.Errorf("signing CA certificate and key do not match")
	}
	return certificate, key, string(certificatePEM), nil
}
