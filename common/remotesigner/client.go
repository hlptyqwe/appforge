// Package remotesigner implements the fixed mTLS protocol used to send an
// aligned APK to a customer-controlled HSM signing gateway.
package remotesigner

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	SchemaVersion          = 1
	defaultMaximumAPKBytes = int64(1 << 30)
	maximumSecretFieldSize = 48 << 10
)

var (
	keyIDPattern       = regexp.MustCompile(`^[A-Za-z0-9._:/-]{1,255}$`)
	fingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Secret struct {
	Endpoint             string `json:"endpoint"`
	KeyID                string `json:"keyId"`
	CACertificatePEM     string `json:"caCertificatePem"`
	ClientCertificatePEM string `json:"clientCertificatePem"`
	ClientPrivateKeyPEM  string `json:"clientPrivateKeyPem"`
	ServerName           string `json:"serverName,omitempty"`
}

func ParseSecret(raw []byte) (*Secret, error) {
	var secret Secret
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&secret); err != nil {
		return nil, errors.New("remote signer secret must be strict JSON")
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	secret.Endpoint = strings.TrimSpace(secret.Endpoint)
	secret.KeyID = strings.TrimSpace(secret.KeyID)
	secret.ServerName = strings.TrimSpace(secret.ServerName)
	if !keyIDPattern.MatchString(secret.KeyID) {
		return nil, errors.New("remote signer keyId is invalid")
	}
	parsed, err := url.Parse(secret.Endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("remote signer endpoint must be an HTTPS origin")
	}
	if strings.ContainsAny(secret.ServerName, " /\\\t\r\n") {
		return nil, errors.New("remote signer serverName is invalid")
	}
	for _, field := range []string{secret.CACertificatePEM, secret.ClientCertificatePEM, secret.ClientPrivateKeyPEM} {
		if strings.TrimSpace(field) == "" || len(field) > maximumSecretFieldSize {
			return nil, errors.New("remote signer mTLS material is incomplete")
		}
	}
	secret.Endpoint = strings.TrimSuffix(secret.Endpoint, "/")
	return &secret, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("remote signer secret must contain one JSON object")
	}
	return nil
}

func (s *Secret) Erase() {
	if s == nil {
		return
	}
	s.CACertificatePEM = ""
	s.ClientCertificatePEM = ""
	s.ClientPrivateKeyPEM = ""
}

type Client struct {
	endpoint string
	keyID    string
	http     *http.Client
	maxBytes int64
	now      func() time.Time
}

func NewClient(secret *Secret, maximumAPKBytes int64) (*Client, error) {
	if secret == nil {
		return nil, errors.New("remote signer secret is required")
	}
	if maximumAPKBytes <= 0 {
		maximumAPKBytes = defaultMaximumAPKBytes
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(secret.CACertificatePEM)) {
		return nil, errors.New("remote signer CA certificate is invalid")
	}
	clientCertificate, err := tls.X509KeyPair([]byte(secret.ClientCertificatePEM), []byte(secret.ClientPrivateKeyPEM))
	if err != nil {
		return nil, errors.New("remote signer client certificate or private key is invalid")
	}
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      roots,
		Certificates: []tls.Certificate{clientCertificate},
		ServerName:   secret.ServerName,
	}
	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 2 * time.Minute,
		ExpectContinueTimeout: time.Second,
		IdleConnTimeout:       30 * time.Second,
	}
	return &Client{
		endpoint: secret.Endpoint,
		keyID:    secret.KeyID,
		maxBytes: maximumAPKBytes,
		now:      time.Now,
		http: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Minute,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return errors.New("remote signer redirects are forbidden")
			},
		},
	}, nil
}

type Info struct {
	SchemaVersion     int    `json:"schemaVersion"`
	KeyID             string `json:"keyId"`
	CertificateSHA256 string `json:"certificateSha256"`
}

func (c *Client) Info(ctx context.Context) (*Info, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/v1/info", nil)
	if err != nil {
		return nil, errors.New("create remote signer info request failed")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("query remote signer info: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote signer info returned HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (16<<10)+1))
	if err != nil || len(raw) == 0 || len(raw) > 16<<10 {
		return nil, errors.New("remote signer info response exceeds the configured limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var info Info
	if err := decoder.Decode(&info); err != nil || ensureJSONEOF(decoder) != nil {
		return nil, errors.New("remote signer info response is invalid")
	}
	info.CertificateSHA256 = strings.ToLower(strings.TrimSpace(info.CertificateSHA256))
	if info.SchemaVersion != SchemaVersion || info.KeyID != c.keyID || !fingerprintPattern.MatchString(info.CertificateSHA256) {
		return nil, errors.New("remote signer info response does not match configured key")
	}
	return &info, nil
}

type SignRequest struct {
	TaskID            int64
	BuilderAttempt    int32
	UnsignedAPKPath   string
	SignedAPKPath     string
	CertificateSHA256 string
	Nonce             string
	Timestamp         time.Time
}

type SignResult struct {
	SizeBytes int64
	SHA256    string
	Nonce     string
	Timestamp time.Time
}

func (c *Client) SignFile(ctx context.Context, input SignRequest) (*SignResult, error) {
	if input.TaskID <= 0 || input.BuilderAttempt <= 0 {
		return nil, errors.New("remote signing task identity is invalid")
	}
	expectedCertificate := strings.ToLower(strings.TrimSpace(input.CertificateSHA256))
	if !fingerprintPattern.MatchString(expectedCertificate) {
		return nil, errors.New("expected remote signing certificate fingerprint is invalid")
	}
	source, err := os.Open(input.UnsignedAPKPath)
	if err != nil {
		return nil, fmt.Errorf("open unsigned APK: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > c.maxBytes {
		return nil, errors.New("unsigned APK is not a bounded regular file")
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, source); err != nil {
		return nil, errors.New("hash unsigned APK failed")
	}
	unsignedSHA := hex.EncodeToString(digest.Sum(nil))
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return nil, errors.New("rewind unsigned APK failed")
	}
	nonce := strings.TrimSpace(input.Nonce)
	if nonce == "" {
		nonce, err = randomNonce()
		if err != nil {
			return nil, err
		}
	}
	if decoded, decodeErr := base64.RawURLEncoding.DecodeString(nonce); decodeErr != nil || len(decoded) < 24 || len(decoded) > 64 {
		return nil, errors.New("remote signing nonce is invalid")
	}
	timestamp := input.Timestamp.UTC()
	if timestamp.IsZero() {
		timestamp = c.now().UTC()
	}
	timestampText := timestamp.Format(time.RFC3339Nano)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/v1/sign-apk", source)
	if err != nil {
		return nil, errors.New("create remote signing request failed")
	}
	request.ContentLength = info.Size()
	request.Header.Set("Content-Type", "application/vnd.android.package-archive")
	request.Header.Set("X-AppForge-Schema-Version", strconv.Itoa(SchemaVersion))
	request.Header.Set("X-AppForge-Task-Id", strconv.FormatInt(input.TaskID, 10))
	request.Header.Set("X-AppForge-Builder-Attempt", strconv.FormatInt(int64(input.BuilderAttempt), 10))
	request.Header.Set("X-AppForge-Key-Id", c.keyID)
	request.Header.Set("X-AppForge-Request-Nonce", nonce)
	request.Header.Set("X-AppForge-Request-Timestamp", timestampText)
	request.Header.Set("X-AppForge-Unsigned-Sha256", unsignedSHA)
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("remote APK signing request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return nil, fmt.Errorf("remote APK signer returned HTTP %d", response.StatusCode)
	}
	responseSignedSHA := strings.ToLower(strings.TrimSpace(response.Header.Get("X-AppForge-Signed-Sha256")))
	if response.Header.Get("X-AppForge-Schema-Version") != strconv.Itoa(SchemaVersion) ||
		response.Header.Get("X-AppForge-Task-Id") != strconv.FormatInt(input.TaskID, 10) ||
		response.Header.Get("X-AppForge-Builder-Attempt") != strconv.FormatInt(int64(input.BuilderAttempt), 10) ||
		response.Header.Get("X-AppForge-Key-Id") != c.keyID ||
		response.Header.Get("X-AppForge-Request-Nonce") != nonce ||
		response.Header.Get("X-AppForge-Request-Timestamp") != timestampText ||
		response.Header.Get("X-AppForge-Unsigned-Sha256") != unsignedSHA ||
		strings.ToLower(response.Header.Get("X-AppForge-Certificate-Sha256")) != expectedCertificate ||
		!fingerprintPattern.MatchString(responseSignedSHA) {
		return nil, errors.New("remote APK signer response binding is invalid")
	}
	if response.ContentLength > c.maxBytes {
		return nil, errors.New("remote signed APK exceeds the configured limit")
	}
	result, err := writeVerifiedResponse(response.Body, input.SignedAPKPath, c.maxBytes, responseSignedSHA)
	if err != nil {
		return nil, err
	}
	result.Nonce = nonce
	result.Timestamp = timestamp
	return result, nil
}

func randomNonce() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", errors.New("generate remote signing nonce failed")
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func writeVerifiedResponse(reader io.Reader, destination string, maximumBytes int64, expectedSHA string) (_ *SignResult, returnErr error) {
	if strings.TrimSpace(destination) == "" {
		return nil, errors.New("signed APK destination is required")
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("signed APK destination already exists")
	}
	directory := filepath.Dir(destination)
	temporary, err := os.CreateTemp(directory, ".remote-signed-*.apk")
	if err != nil {
		return nil, errors.New("create remote signed APK temporary file failed")
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if returnErr != nil {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return nil, errors.New("secure remote signed APK temporary file failed")
	}
	digest := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temporary, digest), io.LimitReader(reader, maximumBytes+1))
	if copyErr != nil || written <= 0 || written > maximumBytes {
		return nil, errors.New("remote signed APK body is invalid or exceeds the configured limit")
	}
	actualSHA := hex.EncodeToString(digest.Sum(nil))
	if actualSHA != expectedSHA {
		return nil, errors.New("remote signed APK SHA-256 mismatch")
	}
	if err := temporary.Sync(); err != nil {
		return nil, errors.New("sync remote signed APK failed")
	}
	if err := temporary.Close(); err != nil {
		return nil, errors.New("close remote signed APK failed")
	}
	// Link within the destination directory is an atomic no-clobber commit. It
	// prevents a concurrent process from replacing an existing output between
	// the Lstat above and final publication.
	if err := os.Link(temporaryName, destination); err != nil {
		return nil, errors.New("commit remote signed APK failed")
	}
	_ = os.Remove(temporaryName)
	return &SignResult{SizeBytes: written, SHA256: actualSHA}, nil
}
