package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const schemaVersion = "1"

type server struct {
	keyID                 string
	certificateSHA256     string
	keystorePath          string
	keyAlias              string
	storePassword         string
	keyPassword           string
	replayDirectory       string
	maximumBytes          int64
	tamperResponse        bool
	responseCertificate   string
	maximumClockSkew      time.Duration
	requestExecutionLimit time.Duration
	responseDelay         time.Duration
}

func main() {
	listen := flag.String("listen", ":9443", "HTTPS listen address")
	serverCertificate := flag.String("tls-cert", "", "server certificate")
	serverKey := flag.String("tls-key", "", "server private key")
	clientCA := flag.String("client-ca", "", "client CA")
	keystore := flag.String("keystore", "", "test signing Keystore")
	keyAlias := flag.String("key-alias", "release", "signing key alias")
	keyID := flag.String("key-id", "android-release", "remote key ID")
	replayDirectory := flag.String("replay-dir", "", "persistent replay marker directory")
	tamperResponse := flag.Bool("tamper-response", false, "tamper response bytes after digest")
	responseCertificate := flag.String("response-certificate-sha256", "", "optional incorrect response fingerprint")
	responseDelay := flag.Duration("response-delay", 0, "optional synthetic response delay")
	flag.Parse()
	storePassword := os.Getenv("APPFORGE_TEST_KEYSTORE_PASSWORD")
	keyPassword := os.Getenv("APPFORGE_TEST_KEY_PASSWORD")
	for name, value := range map[string]string{
		"tls-cert": *serverCertificate, "tls-key": *serverKey, "client-ca": *clientCA,
		"keystore": *keystore, "replay-dir": *replayDirectory,
	} {
		if strings.TrimSpace(value) == "" {
			log.Fatalf("%s is required", name)
		}
	}
	if storePassword == "" || keyPassword == "" || !validKeyID(*keyID) || strings.TrimSpace(*keyAlias) == "" ||
		*responseDelay < 0 || *responseDelay > 5*time.Minute {
		log.Fatal("signer configuration is incomplete")
	}
	if err := os.MkdirAll(*replayDirectory, 0o700); err != nil {
		log.Fatal("create replay directory failed")
	}
	certificateSHA256, err := keystoreCertificateSHA256(*keystore, *keyAlias, storePassword)
	if err != nil {
		log.Fatal(err)
	}
	roots, err := loadCertPool(*clientCA)
	if err != nil {
		log.Fatal(err)
	}
	pair, err := tls.LoadX509KeyPair(*serverCertificate, *serverKey)
	if err != nil {
		log.Fatal("load server TLS identity failed")
	}
	fixture := &server{
		keyID: *keyID, certificateSHA256: certificateSHA256, keystorePath: *keystore, keyAlias: *keyAlias,
		storePassword: storePassword, keyPassword: keyPassword, replayDirectory: *replayDirectory,
		maximumBytes: 128 << 20, tamperResponse: *tamperResponse,
		responseCertificate: strings.ToLower(strings.TrimSpace(*responseCertificate)),
		maximumClockSkew:    5 * time.Minute, requestExecutionLimit: 90 * time.Second,
		responseDelay: *responseDelay,
	}
	httpServer := &http.Server{
		Addr:              *listen,
		Handler:           fixture,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       30 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair},
			ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots,
		},
	}
	log.Printf("remote APK signer acceptance fixture listening on %s keyId=%s certificateSha256=%s", *listen, *keyID, certificateSHA256)
	if err := httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func (s *server) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/v1/info":
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"schemaVersion": 1, "keyId": s.keyID, "certificateSha256": s.certificateSHA256,
		})
	case request.Method == http.MethodPost && request.URL.Path == "/v1/sign-apk":
		s.sign(response, request)
	default:
		http.NotFound(response, request)
	}
}

func (s *server) sign(response http.ResponseWriter, request *http.Request) {
	if request.TLS == nil || len(request.TLS.PeerCertificates) == 0 {
		http.Error(response, "verified client certificate is required", http.StatusUnauthorized)
		return
	}
	taskID, taskErr := strconv.ParseInt(request.Header.Get("X-AppForge-Task-Id"), 10, 64)
	attempt, attemptErr := strconv.ParseInt(request.Header.Get("X-AppForge-Builder-Attempt"), 10, 32)
	timestampText := request.Header.Get("X-AppForge-Request-Timestamp")
	timestamp, timestampErr := time.Parse(time.RFC3339Nano, timestampText)
	nonce := request.Header.Get("X-AppForge-Request-Nonce")
	nonceBytes, nonceErr := base64.RawURLEncoding.DecodeString(nonce)
	unsignedSHA := strings.ToLower(request.Header.Get("X-AppForge-Unsigned-Sha256"))
	if request.Header.Get("X-AppForge-Schema-Version") != schemaVersion || taskErr != nil || taskID <= 0 ||
		attemptErr != nil || attempt <= 0 || request.Header.Get("X-AppForge-Key-Id") != s.keyID ||
		timestampErr != nil || time.Since(timestamp) > s.maximumClockSkew || time.Until(timestamp) > s.maximumClockSkew ||
		nonceErr != nil || len(nonceBytes) < 24 || len(nonceBytes) > 64 || len(unsignedSHA) != 64 {
		http.Error(response, "invalid signing envelope", http.StatusBadRequest)
		return
	}
	if s.responseDelay > 0 {
		select {
		case <-time.After(s.responseDelay):
		case <-request.Context().Done():
			return
		}
	}
	temporary, err := os.MkdirTemp("", "remote-apk-signer-")
	if err != nil {
		http.Error(response, "temporary directory failed", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(temporary)
	unsignedPath := filepath.Join(temporary, "unsigned.apk")
	signedPath := filepath.Join(temporary, "signed.apk")
	unsignedFile, err := os.OpenFile(unsignedPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		http.Error(response, "temporary input failed", http.StatusInternalServerError)
		return
	}
	digest := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(unsignedFile, digest), io.LimitReader(request.Body, s.maximumBytes+1))
	closeErr := unsignedFile.Close()
	if copyErr != nil || closeErr != nil || written <= 0 || written > s.maximumBytes || hex.EncodeToString(digest.Sum(nil)) != unsignedSHA {
		http.Error(response, "unsigned APK digest mismatch", http.StatusBadRequest)
		return
	}
	clientIdentity := sha256.Sum256(request.TLS.PeerCertificates[0].Raw)
	markerDigest := sha256.Sum256([]byte(fmt.Sprintf("%x:%d:%d:%s:%s", clientIdentity, taskID, attempt, nonce, unsignedSHA)))
	markerPath := filepath.Join(s.replayDirectory, hex.EncodeToString(markerDigest[:])+".used")
	marker, err := os.OpenFile(markerPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		http.Error(response, "replayed signing request", http.StatusConflict)
		return
	}
	_ = marker.Close()
	succeeded := false
	defer func() {
		if !succeeded {
			_ = os.Remove(markerPath)
		}
	}()
	ctx, cancel := context.WithTimeout(request.Context(), s.requestExecutionLimit)
	defer cancel()
	command := exec.CommandContext(ctx, "apksigner", "sign", "--ks", s.keystorePath,
		"--ks-key-alias", s.keyAlias, "--ks-pass", "env:APPFORGE_TEST_KEYSTORE_PASSWORD",
		"--key-pass", "env:APPFORGE_TEST_KEY_PASSWORD", "--out", signedPath, unsignedPath)
	command.Env = append(os.Environ(), "APPFORGE_TEST_KEYSTORE_PASSWORD="+s.storePassword, "APPFORGE_TEST_KEY_PASSWORD="+s.keyPassword)
	if output, err := command.CombinedOutput(); err != nil {
		log.Printf("apksigner failed: %v outputBytes=%d", err, len(output))
		http.Error(response, "APK signing failed", http.StatusInternalServerError)
		return
	}
	signed, err := os.ReadFile(signedPath)
	if err != nil || len(signed) == 0 || int64(len(signed)) > s.maximumBytes {
		http.Error(response, "signed APK output failed", http.StatusInternalServerError)
		return
	}
	signedDigest := sha256.Sum256(signed)
	for _, header := range []string{
		"X-AppForge-Schema-Version", "X-AppForge-Task-Id", "X-AppForge-Builder-Attempt",
		"X-AppForge-Key-Id", "X-AppForge-Request-Nonce", "X-AppForge-Request-Timestamp", "X-AppForge-Unsigned-Sha256",
	} {
		response.Header().Set(header, request.Header.Get(header))
	}
	responseCertificate := s.certificateSHA256
	if s.responseCertificate != "" {
		responseCertificate = s.responseCertificate
	}
	response.Header().Set("X-AppForge-Certificate-Sha256", responseCertificate)
	response.Header().Set("X-AppForge-Signed-Sha256", hex.EncodeToString(signedDigest[:]))
	response.Header().Set("Content-Type", "application/vnd.android.package-archive")
	// Signing already consumed the one-time envelope. Keep the replay marker even
	// when the client disconnects before receiving the response, so retrying the
	// same nonce can never invoke the signing key a second time.
	succeeded = true
	if s.tamperResponse {
		signed[len(signed)-1] ^= 0xff
	}
	_, _ = response.Write(signed)
}

func validKeyID(value string) bool {
	if len(value) == 0 || len(value) > 255 {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z') && !(character >= 'A' && character <= 'Z') &&
			!(character >= '0' && character <= '9') && !strings.ContainsRune("._:/-", character) {
			return false
		}
	}
	return true
}

func loadCertPool(path string) (*x509.CertPool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errorsText("read client CA failed")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(content) {
		return nil, errorsText("client CA is invalid")
	}
	return pool, nil
}

func keystoreCertificateSHA256(path, alias, password string) (string, error) {
	command := exec.Command("keytool", "-exportcert", "-keystore", path, "-storepass:env", "APPFORGE_TEST_KEYSTORE_PASSWORD", "-alias", alias)
	command.Env = append(os.Environ(), "APPFORGE_TEST_KEYSTORE_PASSWORD="+password)
	certificate, err := command.Output()
	if err != nil || len(certificate) == 0 {
		return "", errorsText("export signing certificate failed")
	}
	digest := sha256.Sum256(certificate)
	return hex.EncodeToString(digest[:]), nil
}

type errorsText string

func (e errorsText) Error() string { return string(e) }
