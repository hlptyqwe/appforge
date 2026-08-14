package logic

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"appforge/common/storage"
	"appforge/proto/core"
)

type memoryObjectStore struct {
	data []byte
}

func (m memoryObjectStore) PutObject(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (m memoryObjectStore) OpenObject(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(m.data)), nil
}

func (m memoryObjectStore) StatObject(context.Context, string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{Key: "test", Size: int64(len(m.data))}, nil
}

func (m memoryObjectStore) DeleteObject(context.Context, string) error { return nil }

func (m memoryObjectStore) PresignPut(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

func (m memoryObjectStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

func TestVerifyStorageObjectAcceptsAPKWithManifest(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	manifest, err := writer.Create("AndroidManifest.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manifest.Write([]byte("manifest")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	item := &core.StorageObject{
		ObjectKey: "tenants/1/source-apk/test.apk", ObjectType: core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK,
		SizeBytes: int64(buffer.Len()),
	}
	size, checksum, err := VerifyStorageObject(context.Background(), memoryObjectStore{data: buffer.Bytes()}, item)
	if err != nil {
		t.Fatalf("VerifyStorageObject() error = %v", err)
	}
	if size != int64(buffer.Len()) || len(checksum) != 64 {
		t.Fatalf("VerifyStorageObject() size=%d checksum=%q", size, checksum)
	}
}

func TestVerifyStorageObjectRejectsAPKWithoutManifest(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create("classes.dex")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("dex")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	item := &core.StorageObject{
		ObjectKey: "tenants/1/source-apk/test.apk", ObjectType: core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK,
		SizeBytes: int64(buffer.Len()),
	}
	if _, _, err := VerifyStorageObject(context.Background(), memoryObjectStore{data: buffer.Bytes()}, item); err == nil {
		t.Fatal("VerifyStorageObject() expected missing manifest error")
	}
}
