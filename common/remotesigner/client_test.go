package remotesigner

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const testCertificateSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type testPKI struct {
	caPEM         string
	server        tls.Certificate
	clientCertPEM string
	clientKeyPEM  string
	clientCAs     *x509.CertPool
}

func newTestPKI(t *testing.T) testPKI {
	t.Helper()
	now := time.Now().Add(-time.Minute)
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "AppForge Remote Signer Test CA"},
		NotBefore: now, NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCertificate, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	issue := func(serial int64, commonName string, usages []x509.ExtKeyUsage, ips []net.IP) (string, string, tls.Certificate) {
		key, keyErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if keyErr != nil {
			t.Fatal(keyErr)
		}
		template := &x509.Certificate{
			SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: commonName},
			NotBefore: now, NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
			ExtKeyUsage: usages, IPAddresses: ips, DNSNames: []string{"localhost"},
		}
		certificateDER, createErr := x509.CreateCertificate(rand.Reader, template, caCertificate, &key.PublicKey, caKey)
		if createErr != nil {
			t.Fatal(createErr)
		}
		certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
		keyDER, marshalErr := x509.MarshalPKCS8PrivateKey(key)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
		pair, pairErr := tls.X509KeyPair(certificatePEM, keyPEM)
		if pairErr != nil {
			t.Fatal(pairErr)
		}
		return string(certificatePEM), string(keyPEM), pair
	}
	_, _, server := issue(2, "localhost", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, []net.IP{net.ParseIP("127.0.0.1")})
	clientCertPEM, clientKeyPEM, _ := issue(3, "builder-test", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, nil)
	clientCAs := x509.NewCertPool()
	clientCAs.AppendCertsFromPEM(caPEM)
	return testPKI{caPEM: string(caPEM), server: server, clientCertPEM: clientCertPEM, clientKeyPEM: clientKeyPEM, clientCAs: clientCAs}
}

type signerFixture struct {
	mu       sync.Mutex
	replayed map[string]struct{}
	tamper   bool
}

func (f *signerFixture) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/v1/info" && request.Method == http.MethodGet {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(Info{SchemaVersion: SchemaVersion, KeyID: "android-release", CertificateSHA256: testCertificateSHA})
		return
	}
	if request.URL.Path != "/v1/sign-apk" || request.Method != http.MethodPost {
		http.NotFound(response, request)
		return
	}
	nonce := request.Header.Get("X-AppForge-Request-Nonce")
	f.mu.Lock()
	_, duplicate := f.replayed[nonce]
	if !duplicate {
		f.replayed[nonce] = struct{}{}
	}
	f.mu.Unlock()
	if duplicate {
		http.Error(response, "replayed", http.StatusConflict)
		return
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(response, "read", http.StatusBadRequest)
		return
	}
	unsignedDigest := sha256.Sum256(body)
	if request.Header.Get("X-AppForge-Unsigned-Sha256") != hex.EncodeToString(unsignedDigest[:]) {
		http.Error(response, "digest", http.StatusBadRequest)
		return
	}
	signed := append([]byte("signed:"), body...)
	signedDigest := sha256.Sum256(signed)
	for _, header := range []string{
		"X-AppForge-Schema-Version", "X-AppForge-Task-Id", "X-AppForge-Builder-Attempt",
		"X-AppForge-Key-Id", "X-AppForge-Request-Nonce", "X-AppForge-Request-Timestamp", "X-AppForge-Unsigned-Sha256",
	} {
		response.Header().Set(header, request.Header.Get(header))
	}
	response.Header().Set("X-AppForge-Certificate-Sha256", testCertificateSHA)
	response.Header().Set("X-AppForge-Signed-Sha256", hex.EncodeToString(signedDigest[:]))
	if f.tamper {
		signed[len(signed)-1] ^= 0xff
	}
	_, _ = response.Write(signed)
}

func newFixtureClient(t *testing.T, tamper bool) (*Client, func()) {
	t.Helper()
	pki := newTestPKI(t)
	fixture := &signerFixture{replayed: make(map[string]struct{}), tamper: tamper}
	server := httptest.NewUnstartedServer(fixture)
	server.TLS = &tls.Config{
		MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pki.server},
		ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.clientCAs,
	}
	server.StartTLS()
	secret := &Secret{
		Endpoint: server.URL, KeyID: "android-release", CACertificatePEM: pki.caPEM,
		ClientCertificatePEM: pki.clientCertPEM, ClientPrivateKeyPEM: pki.clientKeyPEM,
	}
	client, err := NewClient(secret, 1<<20)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	return client, server.Close
}

func TestParseSecretStrictAndHTTPS(t *testing.T) {
	valid := map[string]string{
		"endpoint": "https://signer.example.com", "keyId": "android/release-1",
		"caCertificatePem": "ca", "clientCertificatePem": "cert", "clientPrivateKeyPem": "key",
	}
	raw, _ := json.Marshal(valid)
	secret, err := ParseSecret(raw)
	if err != nil || secret.Endpoint != "https://signer.example.com" || secret.KeyID != "android/release-1" {
		t.Fatalf("parse valid secret: secret=%+v err=%v", secret, err)
	}
	valid["endpoint"] = "http://signer.example.com"
	raw, _ = json.Marshal(valid)
	if _, err := ParseSecret(raw); err == nil {
		t.Fatal("HTTP endpoint must be rejected")
	}
	if _, err := ParseSecret([]byte(`{"endpoint":"https://signer.example.com","keyId":"key","caCertificatePem":"ca","clientCertificatePem":"cert","clientPrivateKeyPem":"key","unknown":true}`)); err == nil {
		t.Fatal("unknown secret field must be rejected")
	}
}

func TestClientInfoAndSignBinding(t *testing.T) {
	client, closeServer := newFixtureClient(t, false)
	defer closeServer()
	info, err := client.Info(context.Background())
	if err != nil || info.KeyID != "android-release" || info.CertificateSHA256 != testCertificateSHA {
		t.Fatalf("info mismatch: info=%+v err=%v", info, err)
	}
	root := t.TempDir()
	unsignedPath := filepath.Join(root, "unsigned.apk")
	signedPath := filepath.Join(root, "signed.apk")
	if err := os.WriteFile(unsignedPath, []byte("synthetic-apk"), 0o600); err != nil {
		t.Fatal(err)
	}
	nonce := base64.RawURLEncoding.EncodeToString(bytesOf(32, 7))
	timestamp := time.Now().UTC().Truncate(time.Microsecond)
	result, err := client.SignFile(context.Background(), SignRequest{
		TaskID: 17, BuilderAttempt: 2, UnsignedAPKPath: unsignedPath, SignedAPKPath: signedPath,
		CertificateSHA256: testCertificateSHA, Nonce: nonce, Timestamp: timestamp,
	})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(signedPath)
	if err != nil || string(content) != "signed:synthetic-apk" || result.Nonce != nonce || !result.Timestamp.Equal(timestamp) {
		t.Fatalf("signed result mismatch: content=%q result=%+v err=%v", content, result, err)
	}
	if mode := fileMode(t, signedPath); mode != 0o600 {
		t.Fatalf("signed APK mode=%o", mode)
	}
	secondPath := filepath.Join(root, "replayed.apk")
	if _, err := client.SignFile(context.Background(), SignRequest{
		TaskID: 17, BuilderAttempt: 2, UnsignedAPKPath: unsignedPath, SignedAPKPath: secondPath,
		CertificateSHA256: testCertificateSHA, Nonce: nonce, Timestamp: timestamp,
	}); err == nil || !strings.Contains(err.Error(), "HTTP 409") {
		t.Fatalf("replayed nonce must be rejected, err=%v", err)
	}
}

func TestClientRejectsTamperedResponseAndRemovesOutput(t *testing.T) {
	client, closeServer := newFixtureClient(t, true)
	defer closeServer()
	root := t.TempDir()
	unsignedPath := filepath.Join(root, "unsigned.apk")
	signedPath := filepath.Join(root, "signed.apk")
	if err := os.WriteFile(unsignedPath, []byte("synthetic-apk"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := client.SignFile(context.Background(), SignRequest{
		TaskID: 19, BuilderAttempt: 1, UnsignedAPKPath: unsignedPath, SignedAPKPath: signedPath,
		CertificateSHA256: testCertificateSHA,
	})
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("tampered response must fail, err=%v", err)
	}
	if _, statErr := os.Stat(signedPath); !os.IsNotExist(statErr) {
		t.Fatalf("failed output must not exist: %v", statErr)
	}
}

func TestClientRejectsWrongResponseBinding(t *testing.T) {
	pki := newTestPKI(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		digest := sha256.Sum256(body)
		response.Header().Set("X-AppForge-Schema-Version", strconv.Itoa(SchemaVersion))
		response.Header().Set("X-AppForge-Task-Id", request.Header.Get("X-AppForge-Task-Id"))
		response.Header().Set("X-AppForge-Builder-Attempt", request.Header.Get("X-AppForge-Builder-Attempt"))
		response.Header().Set("X-AppForge-Key-Id", "wrong-key")
		response.Header().Set("X-AppForge-Request-Nonce", request.Header.Get("X-AppForge-Request-Nonce"))
		response.Header().Set("X-AppForge-Request-Timestamp", request.Header.Get("X-AppForge-Request-Timestamp"))
		response.Header().Set("X-AppForge-Unsigned-Sha256", request.Header.Get("X-AppForge-Unsigned-Sha256"))
		response.Header().Set("X-AppForge-Certificate-Sha256", testCertificateSHA)
		response.Header().Set("X-AppForge-Signed-Sha256", hex.EncodeToString(digest[:]))
		_, _ = response.Write(body)
	}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pki.server}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.clientCAs}
	server.StartTLS()
	defer server.Close()
	client, err := NewClient(&Secret{Endpoint: server.URL, KeyID: "android-release", CACertificatePEM: pki.caPEM, ClientCertificatePEM: pki.clientCertPEM, ClientPrivateKeyPEM: pki.clientKeyPEM}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	unsignedPath := filepath.Join(root, "unsigned.apk")
	if err := os.WriteFile(unsignedPath, []byte("apk"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = client.SignFile(context.Background(), SignRequest{TaskID: 1, BuilderAttempt: 1, UnsignedAPKPath: unsignedPath, SignedAPKPath: filepath.Join(root, "signed.apk"), CertificateSHA256: testCertificateSHA})
	if err == nil || !strings.Contains(err.Error(), "response binding") {
		t.Fatalf("wrong response key binding must fail, err=%v", err)
	}
}

func TestWriteVerifiedResponseDoesNotOverwriteExisting(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "signed.apk")
	if err := os.WriteFile(destination, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("replacement"))
	if _, err := writeVerifiedResponse(strings.NewReader("replacement"), destination, 1<<20, hex.EncodeToString(digest[:])); err == nil {
		t.Fatal("existing signed APK must not be overwritten")
	}
	content, err := os.ReadFile(destination)
	if err != nil || string(content) != "existing" {
		t.Fatalf("existing output changed: content=%q err=%v", content, err)
	}
}

func TestClientDoesNotInheritEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://proxy.invalid:3128")
	pki := newTestPKI(t)
	client, err := NewClient(&Secret{
		Endpoint:             "https://signer.example.test",
		KeyID:                "android-release",
		CACertificatePEM:     pki.caPEM,
		ClientCertificatePEM: pki.clientCertPEM,
		ClientPrivateKeyPEM:  pki.clientKeyPEM,
	}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := client.http.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatal("remote signer must not inherit HTTP(S) proxy settings")
	}
}

func bytesOf(count int, value byte) []byte {
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}
