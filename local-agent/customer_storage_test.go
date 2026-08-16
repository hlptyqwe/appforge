package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type memoryCustomerStore struct {
	objects      map[string][]byte
	contentTypes map[string]string
	tamperOpen   bool
}

func newMemoryCustomerStore() *memoryCustomerStore {
	return &memoryCustomerStore{objects: map[string][]byte{}, contentTypes: map[string]string{}}
}

func (s *memoryCustomerStore) Put(_ context.Context, key string, reader io.Reader, size int64, contentType string) error {
	value, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if int64(len(value)) != size {
		return errors.New("size mismatch")
	}
	s.objects[key] = append([]byte(nil), value...)
	s.contentTypes[key] = contentType
	return nil
}

func (s *memoryCustomerStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	value, ok := s.objects[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	value = append([]byte(nil), value...)
	if s.tamperOpen && len(value) > 0 {
		value[0] ^= 0xff
	}
	return io.NopCloser(bytes.NewReader(value)), nil
}

func (s *memoryCustomerStore) Stat(_ context.Context, key string) (customerObjectInfo, error) {
	value, ok := s.objects[key]
	if !ok {
		return customerObjectInfo{}, os.ErrNotExist
	}
	return customerObjectInfo{Size: int64(len(value)), ContentType: s.contentTypes[key]}, nil
}

func TestCustomerStorageSecretIsStrictAndPrefixBound(t *testing.T) {
	valid := `{"provider":"minio","endpoint":"https://objects.example","region":"us-east-1","access_key_id":"test-id","access_key_secret":"test-secret","bucket":"synthetic","prefix":"tenants/7/agents/build-a"}`
	secret, err := decodeCustomerStorageSecret(strings.NewReader(valid))
	if err != nil {
		t.Fatal(err)
	}
	secret.erase()
	if secret.AccessKeyID != "" || secret.AccessKeySecret != "" {
		t.Fatal("customer storage credentials were not erased")
	}
	for _, invalid := range []string{
		strings.TrimSuffix(valid, "}") + `,"unknown":"forbidden"}`,
		valid + `{}`,
		strings.Replace(valid, `"provider":"minio"`, `"provider":"ftp"`, 1),
		strings.Replace(valid, `"prefix":"tenants/7/agents/build-a"`, `"prefix":"../escape"`, 1),
	} {
		if _, err := decodeCustomerStorageSecret(strings.NewReader(invalid)); err == nil {
			t.Fatalf("unsafe customer storage Secret accepted: %s", invalid)
		}
	}
}

func TestCustomerStorageReferenceAndSecretFileAreRestricted(t *testing.T) {
	root := t.TempDir()
	secretPath := filepath.Join(root, "customer-storage.json")
	if err := os.WriteFile(secretPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	relative, prefix, err := parseCustomerStorageReference(root,
		"local-file:///customer-storage.json#tenants/7/agents/build-a")
	if err != nil || relative != "customer-storage.json" || prefix != "tenants/7/agents/build-a" {
		t.Fatalf("relative=%q prefix=%q err=%v", relative, prefix, err)
	}
	file, err := openPrivateSecretFile(root, relative)
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	if err := os.Chmod(secretPath, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := openPrivateSecretFile(root, relative); err == nil {
		t.Fatal("group-readable customer storage Secret was accepted")
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := openPrivateSecretFile(root, "linked.json"); err == nil {
		t.Fatal("symlinked customer storage Secret was accepted")
	}
	for _, invalid := range []string{
		"local-file:///customer-storage.json?token=secret#tenants/7/agents/build-a",
		"local-file:///customer-storage.json#tenants/7/agents/../build-a",
		"https://objects.example/customer-storage.json#tenants/7/agents/build-a",
	} {
		if _, _, err := parseCustomerStorageReference(root, invalid); err == nil {
			t.Fatalf("unsafe customer storage reference accepted: %s", invalid)
		}
	}
}

func TestCustomerObjectReferencesAreAgentAndPrefixScoped(t *testing.T) {
	prefix := "tenants/7/agents/build-a"
	digest := strings.Repeat("a", 64)
	key, err := customerInputObjectKey(prefix, 11, 1, digest, "source.apk")
	if err != nil {
		t.Fatal(err)
	}
	reference, err := customerObjectReference(19, key)
	if err != nil {
		t.Fatal(err)
	}
	if parsed, err := parseCustomerObjectReference(reference, 19, prefix); err != nil || parsed != key {
		t.Fatalf("parsed=%q err=%v", parsed, err)
	}
	for _, invalid := range []string{
		strings.Replace(reference, "//19/", "//20/", 1),
		"customer-object://19/tenants/8/agents/build-a/input.apk",
		reference + "?signature=secret",
		"customer-object://19/" + prefix + "/../escape.apk",
	} {
		if _, err := parseCustomerObjectReference(invalid, 19, prefix); err == nil {
			t.Fatalf("unsafe customer object reference accepted: %s", invalid)
		}
	}
}

func TestCustomerObjectTransferReopensAndVerifiesBytes(t *testing.T) {
	ctx := context.Background()
	store := newMemoryCustomerStore()
	payload := []byte("synthetic-customer-apk")
	digest := digestBytes(payload)
	key := "tenants/7/agents/build-a/inputs/apps/11/1/" + digest + ".apk"
	store.objects[key] = append([]byte(nil), payload...)
	store.contentTypes[key] = "application/vnd.android.package-archive"
	target := filepath.Join(t.TempDir(), "source.apk")
	if err := copyVerifiedCustomerObject(ctx, store, key, target, int64(len(payload)), digest); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(actual, payload) {
		t.Fatalf("copied=%q err=%v", actual, err)
	}

	output := filepath.Join(t.TempDir(), "built.apk")
	if err := os.WriteFile(output, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	outputKey := "tenants/7/agents/build-a/tasks/101/attempts/2/built.apk"
	result, err := uploadAndVerifyCustomerObject(ctx, store, outputKey, output, "application/vnd.android.package-archive")
	if err != nil || result.SHA256 != digest || result.SizeBytes != int64(len(payload)) {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	store.tamperOpen = true
	if _, err := uploadAndVerifyCustomerObject(ctx, store, outputKey, output, "application/vnd.android.package-archive"); err == nil {
		t.Fatal("upload whose reopened bytes were tampered was accepted")
	}
}

func TestDownloadCustomerInputsRejectsWrongOwnerAndTamper(t *testing.T) {
	payload := []byte("synthetic-keystore")
	digest := digestBytes(payload)
	key := "tenants/7/agents/build-a/inputs/apps/11/2/" + digest + ".jks"
	reference := "customer-object://19/" + key
	store := newMemoryCustomerStore()
	store.objects[key] = append([]byte(nil), payload...)
	bundle := &buildManifest{Inputs: []buildInput{{Role: "keystore", ObjectID: 91, OriginalName: "release.jks",
		SizeBytes: int64(len(payload)), SHA256: digest, StorageMode: 2, OwnerAgentID: 20, CustomerReference: reference}}}
	if err := downloadCustomerArtifactInputs(context.Background(), store, 19, "tenants/7/agents/build-a", t.TempDir(), bundle); err == nil {
		t.Fatal("customer input owned by another Agent was accepted")
	}
	bundle.Inputs[0].OwnerAgentID = 19
	store.tamperOpen = true
	if err := downloadCustomerArtifactInputs(context.Background(), store, 19, "tenants/7/agents/build-a", t.TempDir(), bundle); err == nil {
		t.Fatal("tampered customer input was accepted")
	}
}
