package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type customerStorageInputResponse struct {
	Data struct {
		ObjectID     int64  `json:"id"`
		SizeBytes    int64  `json:"size_bytes"`
		SHA256       string `json:"sha256"`
		StorageMode  int32  `json:"storage_mode"`
		OwnerAgentID int64  `json:"owner_agent_id"`
	} `json:"data"`
}

const (
	customerStorageProviderMinIO    = "minio"
	customerStorageProviderS3       = "s3"
	customerStorageProviderAliyunOS = "aliyun-oss"
)

type customerStorageSecret struct {
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region,omitempty"`
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	SessionToken    string `json:"session_token,omitempty"`
	Bucket          string `json:"bucket"`
	Prefix          string `json:"prefix"`
	CAFile          string `json:"ca_file,omitempty"`
}

type customerObjectInfo struct {
	Size        int64
	ContentType string
}

type customerObjectStore interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Open(context.Context, string) (io.ReadCloser, error)
	Stat(context.Context, string) (customerObjectInfo, error)
	Delete(context.Context, string) error
}

type minioCustomerStore struct {
	client *minio.Client
	bucket string
}

type aliyunCustomerStore struct {
	bucket *oss.Bucket
}

func resolveCustomerStorage(secretRoot, reference string) (*customerStorageSecret, customerObjectStore, error) {
	secretPath, registeredPrefix, err := parseCustomerStorageReference(secretRoot, reference)
	if err != nil {
		return nil, nil, err
	}
	file, err := openPrivateSecretFile(secretRoot, secretPath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	secret, err := decodeCustomerStorageSecret(file)
	if err != nil {
		return nil, nil, err
	}
	if secret.Prefix != registeredPrefix {
		secret.erase()
		return nil, nil, errors.New("customer storage Secret prefix differs from registered prefix")
	}
	store, err := newCustomerObjectStore(secretRoot, secret)
	if err != nil {
		secret.erase()
		return nil, nil, err
	}
	return secret, store, nil
}

func decodeCustomerStorageSecret(reader io.Reader) (*customerStorageSecret, error) {
	decoder := json.NewDecoder(io.LimitReader(reader, 64<<10))
	decoder.DisallowUnknownFields()
	var secret customerStorageSecret
	if err := decoder.Decode(&secret); err != nil {
		return nil, errors.New("customer storage Secret must be strict JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		secret.erase()
		return nil, errors.New("customer storage Secret contains trailing data")
	}
	secret.Provider = strings.ToLower(strings.TrimSpace(secret.Provider))
	secret.Endpoint = strings.TrimSpace(secret.Endpoint)
	secret.Region = strings.TrimSpace(secret.Region)
	secret.Bucket = strings.TrimSpace(secret.Bucket)
	secret.Prefix = strings.Trim(strings.TrimSpace(secret.Prefix), "/")
	secret.CAFile = strings.TrimSpace(secret.CAFile)
	if secret.AccessKeyID == "" || secret.AccessKeySecret == "" || secret.Bucket == "" ||
		len(secret.AccessKeyID) > 4096 || len(secret.AccessKeySecret) > 4096 || len(secret.SessionToken) > 8192 ||
		!validCustomerObjectKey(secret.Prefix) || len(secret.Prefix) > 300 {
		secret.erase()
		return nil, errors.New("customer storage Secret is incomplete")
	}
	if secret.Provider != customerStorageProviderMinIO && secret.Provider != customerStorageProviderS3 && secret.Provider != customerStorageProviderAliyunOS {
		secret.erase()
		return nil, errors.New("customer storage provider must be minio, s3 or aliyun-oss")
	}
	return &secret, nil
}

func parseCustomerStorageReference(secretRoot, reference string) (string, string, error) {
	if !filepath.IsAbs(strings.TrimSpace(secretRoot)) {
		return "", "", errors.New("local Secret root must be an absolute path")
	}
	value := strings.TrimSpace(reference)
	parsed, err := url.Parse(value)
	if err != nil || strings.ToLower(parsed.Scheme) != "local-file" || parsed.User != nil ||
		(parsed.Host != "" && parsed.Host != "localhost") || parsed.RawQuery != "" || parsed.Fragment == "" ||
		strings.Contains(value, "%") || strings.ContainsAny(value, "\r\n@") {
		return "", "", errors.New("customer storage must use local-file:///name.json#registered/prefix")
	}
	relative := strings.TrimPrefix(filepath.Clean("/"+parsed.Path), "/")
	if relative == "" || relative == "." || !strings.HasSuffix(strings.ToLower(relative), ".json") {
		return "", "", errors.New("customer storage Secret path is invalid")
	}
	prefix := strings.Trim(parsed.Fragment, "/")
	if !validCustomerObjectKey(prefix) || len(prefix) > 300 {
		return "", "", errors.New("customer storage registered prefix is invalid")
	}
	canonical := "local-file:///" + filepath.ToSlash(relative) + "#" + prefix
	if parsed.Host == "localhost" {
		canonical = "local-file://localhost/" + filepath.ToSlash(relative) + "#" + prefix
	}
	if value != canonical {
		return "", "", errors.New("customer storage reference is not canonical")
	}
	return relative, prefix, nil
}

func openPrivateSecretFile(root, relative string) (*os.File, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve local Secret root: %w", err)
	}
	candidate := filepath.Join(resolvedRoot, relative)
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return nil, fmt.Errorf("resolve local Secret: %w", err)
	}
	if resolved != resolvedRoot && !strings.HasPrefix(resolved, resolvedRoot+string(filepath.Separator)) {
		return nil, errors.New("local Secret escapes configured root")
	}
	info, err := os.Lstat(candidate)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("local Secret must be a private regular file and must not be a symlink")
	}
	return os.Open(resolved)
}

func newCustomerObjectStore(secretRoot string, secret *customerStorageSecret) (customerObjectStore, error) {
	parsed, err := url.Parse(secret.Endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, errors.New("customer storage endpoint must be an HTTP(S) origin without credentials")
	}
	switch secret.Provider {
	case customerStorageProviderMinIO, customerStorageProviderS3:
		region := secret.Region
		if region == "" {
			region = "us-east-1"
		}
		transport, err := customerStorageTransport(secretRoot, secret.CAFile)
		if err != nil {
			return nil, err
		}
		client, err := minio.New(parsed.Host, &minio.Options{
			Creds:  credentials.NewStaticV4(secret.AccessKeyID, secret.AccessKeySecret, secret.SessionToken),
			Secure: parsed.Scheme == "https", Region: region, Transport: transport,
		})
		if err != nil {
			return nil, fmt.Errorf("initialize S3-compatible customer storage: %w", err)
		}
		return &minioCustomerStore{client: client, bucket: secret.Bucket}, nil
	case customerStorageProviderAliyunOS:
		if secret.CAFile != "" {
			return nil, errors.New("custom CA is currently supported only for S3-compatible customer storage")
		}
		options := make([]oss.ClientOption, 0, 1)
		if secret.SessionToken != "" {
			options = append(options, oss.SecurityToken(secret.SessionToken))
		}
		client, err := oss.New(secret.Endpoint, secret.AccessKeyID, secret.AccessKeySecret, options...)
		if err != nil {
			return nil, fmt.Errorf("initialize Aliyun OSS customer storage: %w", err)
		}
		bucket, err := client.Bucket(secret.Bucket)
		if err != nil {
			return nil, fmt.Errorf("open Aliyun OSS customer bucket: %w", err)
		}
		return &aliyunCustomerStore{bucket: bucket}, nil
	default:
		return nil, errors.New("customer storage provider is unsupported")
	}
}

func customerStorageTransport(secretRoot, caFile string) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if caFile == "" {
		return transport, nil
	}
	file, err := openPrivateSecretFile(secretRoot, strings.TrimPrefix(filepath.Clean("/"+caFile), "/"))
	if err != nil {
		return nil, fmt.Errorf("open customer storage CA: %w", err)
	}
	defer file.Close()
	caPEM, err := io.ReadAll(io.LimitReader(file, 1<<20))
	if err != nil {
		return nil, err
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("customer storage CA is invalid")
	}
	transport.TLSClientConfig.RootCAs = pool
	return transport, nil
}

func (s *minioCustomerStore) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	if err := validateCustomerStoreKey(key); err != nil {
		return err
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *minioCustomerStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := validateCustomerStoreKey(key); err != nil {
		return nil, err
	}
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := object.Stat(); err != nil {
		_ = object.Close()
		return nil, err
	}
	return object, nil
}

func (s *minioCustomerStore) Stat(ctx context.Context, key string) (customerObjectInfo, error) {
	if err := validateCustomerStoreKey(key); err != nil {
		return customerObjectInfo{}, err
	}
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	return customerObjectInfo{Size: info.Size, ContentType: info.ContentType}, err
}

func (s *minioCustomerStore) Delete(ctx context.Context, key string) error {
	if err := validateCustomerStoreKey(key); err != nil {
		return err
	}
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *aliyunCustomerStore) Put(ctx context.Context, key string, reader io.Reader, _ int64, contentType string) error {
	if err := validateCustomerStoreKey(key); err != nil {
		return err
	}
	return s.bucket.PutObject(key, reader, oss.WithContext(ctx), oss.ContentType(contentType))
}

func (s *aliyunCustomerStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := validateCustomerStoreKey(key); err != nil {
		return nil, err
	}
	return s.bucket.GetObject(key, oss.WithContext(ctx))
}

func (s *aliyunCustomerStore) Stat(ctx context.Context, key string) (customerObjectInfo, error) {
	if err := validateCustomerStoreKey(key); err != nil {
		return customerObjectInfo{}, err
	}
	metadata, err := s.bucket.GetObjectDetailedMeta(key, oss.WithContext(ctx))
	if err != nil {
		return customerObjectInfo{}, err
	}
	size, err := strconv.ParseInt(metadata.Get("Content-Length"), 10, 64)
	if err != nil {
		return customerObjectInfo{}, errors.New("customer object Content-Length is invalid")
	}
	return customerObjectInfo{Size: size, ContentType: metadata.Get("Content-Type")}, nil
}

func (s *aliyunCustomerStore) Delete(ctx context.Context, key string) error {
	if err := validateCustomerStoreKey(key); err != nil {
		return err
	}
	return s.bucket.DeleteObject(key, oss.WithContext(ctx))
}

func isCustomerObjectNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	minioError := minio.ToErrorResponse(err)
	if minioError.StatusCode == http.StatusNotFound || minioError.Code == "NoSuchKey" || minioError.Code == "NoSuchObject" {
		return true
	}
	var ossError oss.ServiceError
	return errors.As(err, &ossError) && ossError.StatusCode == http.StatusNotFound
}

const customerStorageProbeConfirmation = "SYNTHETIC_WRITE_READ_DELETE"

type customerStorageProbeMetrics struct {
	ObjectSizeBytes           int64  `json:"objectSizeBytes"`
	ObjectSHA256              string `json:"objectSha256"`
	ObjectKeyFingerprint      string `json:"objectKeyFingerprintSha256"`
	UploadMilliseconds        int64  `json:"uploadMilliseconds"`
	StatMilliseconds          int64  `json:"statMilliseconds"`
	DownloadMilliseconds      int64  `json:"downloadMilliseconds"`
	DeleteMilliseconds        int64  `json:"deleteMilliseconds"`
	DeleteConfirmed           bool   `json:"deleteConfirmed"`
	ExistingObjectsRead       int    `json:"existingObjectsRead"`
	BucketOrPolicyMutationRun bool   `json:"bucketOrPolicyMutationRun"`
}

type customerStorageProbeEvidence struct {
	SchemaVersion   int                         `json:"schemaVersion"`
	EvidenceType    string                      `json:"evidenceType"`
	AcceptedAt      string                      `json:"acceptedAt"`
	Result          string                      `json:"result"`
	EnvironmentKind string                      `json:"environmentKind"`
	Provider        string                      `json:"provider"`
	Runtime         map[string]any              `json:"runtime"`
	Target          map[string]string           `json:"target"`
	Probe           customerStorageProbeMetrics `json:"probe"`
	Verified        []string                    `json:"verified"`
	Limitations     []string                    `json:"limitations"`
}

func customerStorageProbeCommand(args []string) error {
	flags := flag.NewFlagSet("customer-storage-probe", flag.ContinueOnError)
	stateDir := flags.String("state-dir", defaultStateDir(), "Agent local state directory")
	secretRoot := flags.String("secret-root", "/etc/appforge/local-secrets", "absolute Local Agent Secret root")
	report := flags.String("report", "-", "absolute JSON evidence path or - for stdout")
	environmentKind := flags.String("environment-kind", "", "fixture or customer-test")
	sizeBytes := flags.Int64("size-bytes", 1<<20, "synthetic probe object size in bytes")
	timeout := flags.Duration("timeout", 2*time.Minute, "total object storage probe timeout")
	confirmation := flags.String("confirm-synthetic-only", "", "required synthetic write/read/delete confirmation")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *confirmation != customerStorageProbeConfirmation {
		return errors.New("confirm-synthetic-only must explicitly authorize the synthetic write/read/delete probe")
	}
	if *environmentKind != "fixture" && *environmentKind != "customer-test" {
		return errors.New("environment-kind must be fixture or customer-test")
	}
	if *sizeBytes < 1024 || *sizeBytes > 64<<20 {
		return errors.New("size-bytes must be between 1024 and 67108864")
	}
	if *timeout < 10*time.Second || *timeout > 30*time.Minute {
		return errors.New("timeout must be between 10s and 30m")
	}
	absoluteStateDir, err := filepath.Abs(*stateDir)
	if err != nil {
		return err
	}
	current, err := loadState(absoluteStateDir)
	if err != nil {
		return err
	}
	if err := validateLocalState(absoluteStateDir, &current); err != nil {
		return err
	}
	if strings.TrimSpace(current.CustomerStorageRef) == "" {
		return errors.New("Agent is not registered for customer storage")
	}
	secret, store, err := resolveCustomerStorage(*secretRoot, current.CustomerStorageRef)
	if err != nil {
		return err
	}
	defer secret.erase()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	metrics, err := executeCustomerStorageProbe(ctx, store, secret.Prefix, *sizeBytes)
	if err != nil {
		return fmt.Errorf("customer storage synthetic probe failed: %w", err)
	}
	environmentLimitation := "customer-declared-test-environment"
	if *environmentKind == "fixture" {
		environmentLimitation = "temporary-provider-fixture-not-customer-environment"
	}
	evidence := customerStorageProbeEvidence{
		SchemaVersion:   1,
		EvidenceType:    "v7-customer-object-storage-site-probe",
		AcceptedAt:      time.Now().UTC().Format(time.RFC3339),
		Result:          "passed",
		EnvironmentKind: *environmentKind,
		Provider:        secret.Provider,
		Runtime: map[string]any{
			"goos":            runtime.GOOS,
			"goarch":          runtime.GOARCH,
			"agentVersion":    version,
			"protocolVersion": protocolVersion,
		},
		Target: map[string]string{
			"endpointOriginSha256":   digestBytes([]byte(strings.ToLower(secret.Endpoint))),
			"bucketSha256":           digestBytes([]byte(secret.Bucket)),
			"registeredPrefixSha256": digestBytes([]byte(secret.Prefix)),
		},
		Probe: metrics,
		Verified: []string{
			"registered-local-file-secret-resolved",
			"synthetic-object-uploaded-inside-registered-prefix",
			"object-stat-size-verified",
			"full-object-sha256-verified-after-reopen",
			"exact-synthetic-object-deleted-and-absence-confirmed",
			"no-existing-object-list-or-read",
			"no-bucket-or-policy-mutation",
		},
		Limitations: []string{
			"synthetic-object-only",
			"does-not-validate-control-plane-registration-or-full-apk-build",
			"does-not-access-existing-customer-data",
			environmentLimitation,
		},
	}
	encoded, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if *report == "-" {
		_, err = os.Stdout.Write(encoded)
		return err
	}
	if !filepath.IsAbs(*report) {
		return errors.New("report must be an absolute path or -")
	}
	file, err := os.OpenFile(*report, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(encoded); err != nil {
		_ = file.Close()
		_ = os.Remove(*report)
		return err
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(*report)
		return err
	}
	return os.Chmod(*report, 0o600)
}

func executeCustomerStorageProbe(ctx context.Context, store customerObjectStore, prefix string, sizeBytes int64) (
	metrics customerStorageProbeMetrics, returnErr error) {
	if store == nil || !validCustomerObjectKey(prefix) || sizeBytes < 1024 || sizeBytes > 64<<20 {
		return metrics, errors.New("customer storage probe configuration is invalid")
	}
	nonce := newNonce()
	key := path.Join(prefix, "acceptance", "storage-probe", nonce+".bin")
	seed := []byte("appforge-v7-customer-storage-synthetic-probe-" + nonce + "\n")
	payload := bytes.Repeat(seed, int(sizeBytes)/len(seed)+1)[:int(sizeBytes)]
	digest := digestBytes(payload)
	metrics.ObjectSizeBytes = sizeBytes
	metrics.ObjectSHA256 = digest
	metrics.ObjectKeyFingerprint = digestBytes([]byte(key))
	metrics.ExistingObjectsRead = 0
	metrics.BucketOrPolicyMutationRun = false
	uploaded := false
	defer func() {
		if !uploaded {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if cleanupErr := store.Delete(cleanupCtx, key); cleanupErr != nil {
			if returnErr == nil {
				returnErr = errors.New("cleanup synthetic customer object failed")
			} else {
				returnErr = fmt.Errorf("%w; cleanup synthetic customer object failed", returnErr)
			}
		}
	}()

	started := time.Now()
	uploaded = true
	if err := store.Put(ctx, key, bytes.NewReader(payload), sizeBytes, "application/octet-stream"); err != nil {
		return metrics, errors.New("upload synthetic customer object")
	}
	metrics.UploadMilliseconds = time.Since(started).Milliseconds()

	started = time.Now()
	info, err := store.Stat(ctx, key)
	if err != nil || info.Size != sizeBytes {
		return metrics, errors.New("stat synthetic customer object size")
	}
	metrics.StatMilliseconds = time.Since(started).Milliseconds()

	started = time.Now()
	reader, err := store.Open(ctx, key)
	if err != nil {
		return metrics, errors.New("open synthetic customer object")
	}
	hasher := sha256.New()
	written, readErr := io.Copy(hasher, io.LimitReader(reader, sizeBytes+1))
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil || written != sizeBytes || hex.EncodeToString(hasher.Sum(nil)) != digest {
		return metrics, errors.New("verify synthetic customer object bytes")
	}
	metrics.DownloadMilliseconds = time.Since(started).Milliseconds()

	started = time.Now()
	if err := store.Delete(ctx, key); err != nil {
		return metrics, errors.New("delete synthetic customer object")
	}
	for {
		if _, err := store.Stat(ctx, key); err != nil {
			if !isCustomerObjectNotFound(err) {
				return metrics, errors.New("confirm synthetic customer object deletion with authoritative not-found")
			}
			metrics.DeleteConfirmed = true
			metrics.DeleteMilliseconds = time.Since(started).Milliseconds()
			uploaded = false
			return metrics, nil
		}
		if !wait(ctx, 250*time.Millisecond) {
			return metrics, errors.New("confirm synthetic customer object deletion")
		}
	}
}

func customerInputObjectKey(prefix string, appID int64, objectType int32, digest, filename string) (string, error) {
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if extension == "" || len(extension) > 16 || !validAgentSHA256(digest) || appID <= 0 || objectType <= 0 {
		return "", errors.New("customer input identity is invalid")
	}
	return path.Join(prefix, "inputs", "apps", strconv.FormatInt(appID, 10), strconv.FormatInt(int64(objectType), 10), strings.ToLower(digest)+extension), nil
}

func customerOutputObjectKey(prefix string, item *task, role string) (string, error) {
	if item == nil || item.ID <= 0 || item.BuilderAttempt <= 0 {
		return "", errors.New("customer output task identity is invalid")
	}
	name := ""
	switch role {
	case "built_apk":
		name = "built.apk"
	case "build_log":
		name = "build.log"
	default:
		return "", errors.New("customer output role is invalid")
	}
	return path.Join(prefix, "tasks", strconv.FormatInt(item.ID, 10), "attempts", strconv.FormatInt(int64(item.BuilderAttempt), 10), name), nil
}

func customerObjectReference(agentID int64, key string) (string, error) {
	if agentID <= 0 || !validCustomerObjectKey(key) {
		return "", errors.New("customer object reference identity is invalid")
	}
	return fmt.Sprintf("customer-object://%d/%s", agentID, key), nil
}

func parseCustomerObjectReference(reference string, agentID int64, prefix string) (string, error) {
	value := strings.TrimSpace(reference)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "customer-object" || parsed.Host != strconv.FormatInt(agentID, 10) ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(value, "%") {
		return "", errors.New("customer object reference is invalid")
	}
	key := strings.TrimPrefix(parsed.Path, "/")
	if !validCustomerObjectKey(key) || (key != prefix && !strings.HasPrefix(key, prefix+"/")) ||
		value != fmt.Sprintf("customer-object://%d/%s", agentID, key) {
		return "", errors.New("customer object reference is outside the registered prefix")
	}
	return key, nil
}

func copyVerifiedCustomerObject(ctx context.Context, store customerObjectStore, key, target string, expectedSize int64, expectedDigest string) error {
	if expectedSize <= 0 || !validAgentSHA256(expectedDigest) {
		return errors.New("customer object integrity metadata is invalid")
	}
	info, err := store.Stat(ctx, key)
	if err != nil || info.Size != expectedSize {
		return errors.New("customer object size changed or object is unavailable")
	}
	reader, err := store.Open(ctx, key)
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(reader, expectedSize+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || written != expectedSize || hex.EncodeToString(hasher.Sum(nil)) != strings.ToLower(expectedDigest) {
		_ = os.Remove(target)
		return errors.New("copied customer object size or SHA-256 mismatch")
	}
	return nil
}

func verifyCustomerObject(ctx context.Context, store customerObjectStore, key string, expectedSize int64, expectedDigest string) error {
	if expectedSize <= 0 || !validAgentSHA256(expectedDigest) {
		return errors.New("customer object integrity metadata is invalid")
	}
	info, err := store.Stat(ctx, key)
	if err != nil || info.Size != expectedSize {
		return errors.New("customer object size changed or object is unavailable")
	}
	reader, err := store.Open(ctx, key)
	if err != nil {
		return err
	}
	defer reader.Close()
	hasher := sha256.New()
	written, err := io.Copy(hasher, io.LimitReader(reader, expectedSize+1))
	if err != nil || written != expectedSize || hex.EncodeToString(hasher.Sum(nil)) != strings.ToLower(expectedDigest) {
		return errors.New("customer object size or SHA-256 mismatch")
	}
	return nil
}

func uploadAndVerifyCustomerObject(ctx context.Context, store customerObjectStore, key, localPath, contentType string) (*artifactUploadResponse, error) {
	size, digest, err := localAgentFileDigest(localPath)
	if err != nil || size <= 0 {
		return nil, errors.New("customer output file is invalid")
	}
	file, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	putErr := store.Put(ctx, key, file, size, contentType)
	closeErr := file.Close()
	if putErr != nil {
		return nil, putErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := verifyCustomerObject(ctx, store, key, size, digest); err != nil {
		return nil, fmt.Errorf("reopen customer object: %w", err)
	}
	return &artifactUploadResponse{SHA256: digest, SizeBytes: size}, nil
}

func downloadCustomerArtifactInputs(ctx context.Context, store customerObjectStore, agentID int64, prefix, workDir string, bundle *buildManifest) error {
	if bundle == nil || len(bundle.Inputs) == 0 {
		return errors.New("customer Artifact input manifest is empty")
	}
	inputDir := filepath.Join(workDir, "inputs")
	if err := os.RemoveAll(inputDir); err != nil {
		return err
	}
	if err := os.MkdirAll(inputDir, 0o700); err != nil {
		return err
	}
	for index := range bundle.Inputs {
		input := &bundle.Inputs[index]
		if input.StorageMode != 2 || input.OwnerAgentID != agentID {
			return fmt.Errorf("%s customer input ownership mismatch", input.Role)
		}
		key, err := parseCustomerObjectReference(input.CustomerReference, agentID, prefix)
		if err != nil {
			return fmt.Errorf("%s customer reference: %w", input.Role, err)
		}
		extension := strings.ToLower(filepath.Ext(strings.TrimSpace(input.OriginalName)))
		if len(extension) > 16 {
			extension = ""
		}
		role := safeFileSuffix(input.Role)
		if role == "" {
			role = "input"
		}
		target := filepath.Join(inputDir, fmt.Sprintf("%02d-%s-%d%s", index, role, input.ObjectID, extension))
		if err := copyVerifiedCustomerObject(ctx, store, key, target, input.SizeBytes, input.SHA256); err != nil {
			return fmt.Errorf("%s customer input: %w", input.Role, err)
		}
		input.LocalPath = target
	}
	return nil
}

func uploadCustomerArtifactOutput(ctx context.Context, store customerObjectStore, agentID int64, prefix string,
	item *task, workDir, role, artifactPath string) (*artifactUploadResponse, error) {
	absolute, err := privateArtifactOutputPath(workDir, artifactPath)
	if err != nil {
		return nil, err
	}
	key, err := customerOutputObjectKey(prefix, item, role)
	if err != nil {
		return nil, err
	}
	contentType := "text/plain"
	if role == "built_apk" {
		contentType = "application/vnd.android.package-archive"
	}
	result, err := uploadAndVerifyCustomerObject(ctx, store, key, absolute, contentType)
	if err != nil {
		return nil, err
	}
	result.Reference, err = customerObjectReference(agentID, key)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func customerStorageSecretImportCommand(args []string) error {
	flags := flag.NewFlagSet("customer-storage-secret-import", flag.ContinueOnError)
	secretRoot := flags.String("secret-root", "/etc/appforge/local-secrets", "absolute Local Agent Secret root")
	name := flags.String("name", "", "customer storage Secret JSON filename")
	inputStdin := flags.Bool("input-stdin", false, "read strict customer storage Secret JSON from standard input")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !*inputStdin || !filepath.IsAbs(*secretRoot) {
		return errors.New("input-stdin and an absolute Secret root are required")
	}
	cleanName := filepath.Base(strings.TrimSpace(*name))
	if cleanName == "" || cleanName == "." || cleanName != strings.TrimSpace(*name) || !strings.HasSuffix(strings.ToLower(cleanName), ".json") {
		return errors.New("Secret name must be a single .json filename")
	}
	if err := os.MkdirAll(*secretRoot, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(*secretRoot, 0o700); err != nil {
		return err
	}
	secret, err := decodeCustomerStorageSecret(os.Stdin)
	if err != nil {
		return err
	}
	defer secret.erase()
	if _, err := newCustomerObjectStore(*secretRoot, secret); err != nil {
		return err
	}
	encoded, err := json.Marshal(secret)
	if err != nil {
		return err
	}
	if err := writePrivateFile(filepath.Join(*secretRoot, cleanName), encoded); err != nil {
		return err
	}
	fmt.Printf("Customer storage Secret imported: local-file:///%s#%s\n", cleanName, secret.Prefix)
	return nil
}

func customerStorageImportCommand(args []string) error {
	flags := flag.NewFlagSet("customer-storage-import", flag.ContinueOnError)
	stateDir := flags.String("state-dir", defaultStateDir(), "Agent local state directory")
	secretRoot := flags.String("secret-root", "/etc/appforge/local-secrets", "absolute Local Agent Secret root")
	appID := flags.Int64("app-id", 0, "authorized application ID")
	objectTypeName := flags.String("object-type", "", "source-apk, keystore, brand-logo, brand-splash or template-file")
	inputPath := flags.String("input", "", "local input file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	absoluteStateDir, err := filepath.Abs(*stateDir)
	if err != nil {
		return err
	}
	current, err := loadState(absoluteStateDir)
	if err != nil {
		return err
	}
	if err := validateLocalState(absoluteStateDir, &current); err != nil {
		return err
	}
	if strings.TrimSpace(current.CustomerStorageRef) == "" {
		return errors.New("Agent is not registered for customer storage")
	}
	objectType, contentType, err := customerInputType(*objectTypeName, *inputPath)
	if err != nil || *appID <= 0 {
		return errors.New("app-id, supported object-type and input are required")
	}
	absoluteInput, err := filepath.Abs(*inputPath)
	if err != nil {
		return err
	}
	info, err := os.Lstat(absoluteInput)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 {
		return errors.New("customer input must be a non-empty regular file and must not be a symlink")
	}
	if objectType == 2 && info.Mode().Perm()&0o077 != 0 {
		return errors.New("Keystore input must not be accessible by group or others")
	}
	if err := validateCustomerInputSize(objectType, info.Size()); err != nil {
		return err
	}
	size, digest, err := localAgentFileDigest(absoluteInput)
	if err != nil {
		return err
	}
	secret, store, err := resolveCustomerStorage(*secretRoot, current.CustomerStorageRef)
	if err != nil {
		return err
	}
	defer secret.erase()
	key, err := customerInputObjectKey(secret.Prefix, *appID, objectType, digest, filepath.Base(absoluteInput))
	if err != nil {
		return err
	}
	file, err := os.Open(absoluteInput)
	if err != nil {
		return err
	}
	putErr := store.Put(context.Background(), key, file, size, contentType)
	closeErr := file.Close()
	if putErr != nil {
		return putErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := verifyCustomerObject(context.Background(), store, key, size, digest); err != nil {
		return fmt.Errorf("reopen imported customer object: %w", err)
	}
	reference, err := customerObjectReference(current.AgentID, key)
	if err != nil {
		return err
	}
	client, err := mtlsClient(&current)
	if err != nil {
		return err
	}
	var response customerStorageInputResponse
	payload := map[string]any{"app_id": *appID, "object_type": objectType, "object_reference": reference,
		"original_name": filepath.Base(absoluteInput), "content_type": contentType, "size_bytes": size, "sha256": digest}
	if err := postAuthenticatedJSON(context.Background(), client, &current, absoluteStateDir,
		current.GatewayURL+"/v1/customer-storage/inputs", payload, &response); err != nil {
		return err
	}
	if response.Data.ObjectID <= 0 || response.Data.SizeBytes != size || response.Data.SHA256 != digest ||
		response.Data.StorageMode != 2 || response.Data.OwnerAgentID != current.AgentID {
		return errors.New("customer storage input registration response is invalid")
	}
	fmt.Printf("Customer object registered: object_id=%d size=%d sha256=%s reference=%s\n",
		response.Data.ObjectID, size, digest, reference)
	return nil
}

func customerInputType(value, inputPath string) (int32, string, error) {
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(inputPath)))
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "source-apk":
		if extension != ".apk" {
			return 0, "", errors.New("source APK must use .apk extension")
		}
		return 1, "application/vnd.android.package-archive", nil
	case "keystore":
		if extension != ".jks" && extension != ".keystore" && extension != ".p12" && extension != ".pfx" {
			return 0, "", errors.New("Keystore extension is unsupported")
		}
		return 2, "application/octet-stream", nil
	case "brand-logo":
		if extension != ".png" && extension != ".webp" {
			return 0, "", errors.New("brand logo must be PNG or WebP")
		}
		return 5, "image/" + strings.TrimPrefix(extension, "."), nil
	case "brand-splash":
		if extension != ".png" && extension != ".webp" {
			return 0, "", errors.New("brand splash must be PNG or WebP")
		}
		return 6, "image/" + strings.TrimPrefix(extension, "."), nil
	case "template-file":
		return 7, "application/octet-stream", nil
	default:
		return 0, "", errors.New("customer input object type is unsupported")
	}
}

func validateCustomerInputSize(objectType int32, size int64) error {
	maximum := int64(2 * 1024 * 1024)
	switch objectType {
	case 1:
		maximum = 2 * 1024 * 1024 * 1024
	case 2, 6:
		maximum = 10 * 1024 * 1024
	case 5:
		maximum = 5 * 1024 * 1024
	case 7:
	default:
		return errors.New("customer input object type is unsupported")
	}
	if size <= 0 || size > maximum {
		return errors.New("customer input exceeds the allowed size")
	}
	return nil
}

func validateCustomerStoreKey(key string) error {
	if !validCustomerObjectKey(key) || len(key) > 480 {
		return errors.New("customer object key is invalid")
	}
	return nil
}

func validCustomerObjectKey(key string) bool {
	if key == "" || strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") || path.Clean(key) != key || strings.Contains(key, "..") {
		return false
	}
	for _, current := range key {
		if current >= 'a' && current <= 'z' || current >= 'A' && current <= 'Z' || current >= '0' && current <= '9' ||
			current == '.' || current == '_' || current == '-' || current == '/' {
			continue
		}
		return false
	}
	return true
}

func (secret *customerStorageSecret) erase() {
	if secret == nil {
		return
	}
	secret.AccessKeyID = ""
	secret.AccessKeySecret = ""
	secret.SessionToken = ""
}
