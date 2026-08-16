package logic

import (
	"database/sql"
	"strings"
	"testing"

	"appforge/proto/core"
	"appforge/services/core/models"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestParseControlPlaneStorageReference(t *testing.T) {
	for _, test := range []struct {
		name      string
		reference string
		want      int64
		code      codes.Code
	}{
		{name: "valid", reference: "storage-object://42", want: 42, code: codes.OK},
		{name: "foreign scheme", reference: "https://storage.example/42", code: codes.InvalidArgument},
		{name: "credentials", reference: "storage-object://user@42", code: codes.InvalidArgument},
		{name: "path", reference: "storage-object://42/file.apk", code: codes.InvalidArgument},
		{name: "query", reference: "storage-object://42?token=secret", code: codes.InvalidArgument},
		{name: "non canonical", reference: "storage-object://0042", code: codes.InvalidArgument},
		{name: "zero", reference: "storage-object://0", code: codes.InvalidArgument},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseControlPlaneStorageReference(test.reference)
			if status.Code(err) != test.code || got != test.want {
				t.Fatalf("got id=%d code=%v err=%v", got, status.Code(err), err)
			}
		})
	}
}

func TestValidateControlPlaneStorageObject(t *testing.T) {
	digest := strings.Repeat("a", 64)
	task := &models.TBuildTask{Id: 71, TenantId: 7, AppId: 11, BuilderAttempt: 3}
	valid := func() *models.TStorageObject {
		return &models.TStorageObject{Id: 91, TenantId: 7, AppId: 11,
			ObjectType: int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK), SizeBytes: 128,
			Sha256: sql.NullString{String: digest, Valid: true}, Status: storageStatusReady,
			StorageMode: int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE)}
	}
	if err := validateControlPlaneStorageObject(valid(), task, core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK, digest, 128); err != nil {
		t.Fatalf("valid object rejected: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*models.TStorageObject)
		kind   core.HybridArtifactType
		code   codes.Code
	}{
		{name: "tenant", mutate: func(item *models.TStorageObject) { item.TenantId++ }, kind: core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK, code: codes.PermissionDenied},
		{name: "application", mutate: func(item *models.TStorageObject) { item.AppId++ }, kind: core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK, code: codes.PermissionDenied},
		{name: "object type", mutate: func(item *models.TStorageObject) {
			item.ObjectType = int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE)
		}, kind: core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK, code: codes.FailedPrecondition},
		{name: "upload incomplete", mutate: func(item *models.TStorageObject) { item.Status = storageStatusUploading }, kind: core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK, code: codes.FailedPrecondition},
		{name: "size", mutate: func(item *models.TStorageObject) { item.SizeBytes++ }, kind: core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK, code: codes.FailedPrecondition},
		{name: "digest", mutate: func(item *models.TStorageObject) { item.Sha256.String = strings.Repeat("b", 64) }, kind: core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK, code: codes.FailedPrecondition},
		{name: "offline type", mutate: func(*models.TStorageObject) {}, kind: core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_OFFLINE_TASK_PACKAGE, code: codes.InvalidArgument},
	} {
		t.Run(test.name, func(t *testing.T) {
			item := valid()
			test.mutate(item)
			if code := status.Code(validateControlPlaneStorageObject(item, task, test.kind, digest, 128)); code != test.code {
				t.Fatalf("code=%v, want=%v", code, test.code)
			}
		})
	}
}

func TestCustomerStorageReferencesAreCanonicalAndScoped(t *testing.T) {
	secretPath, prefix, err := parseCustomerStorageDescriptor(
		"local-file:///customer-storage.json#tenants/7/agents/build-a", 7, "build-a")
	if err != nil {
		t.Fatal(err)
	}
	if secretPath != "/customer-storage.json" || prefix != "tenants/7/agents/build-a" {
		t.Fatalf("descriptor path=%q prefix=%q", secretPath, prefix)
	}
	key := customerInputObjectKey(prefix, 11, core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK, strings.Repeat("a", 64), "source.apk")
	reference := customerObjectReference(19, key)
	parsed, err := parseCustomerObjectReference(reference, 19, prefix)
	if err != nil || parsed != key {
		t.Fatalf("parsed=%q err=%v", parsed, err)
	}

	for _, invalid := range []string{
		"local-file:///customer-storage.json#tenants/8/agents/build-a",
		"local-file:///customer-storage.json?token=secret#tenants/7/agents/build-a",
		"local-file:///customer-storage.json#tenants/7/agents/../build-a",
	} {
		if _, _, err := parseCustomerStorageDescriptor(invalid, 7, "build-a"); err == nil {
			t.Fatalf("invalid descriptor accepted: %s", invalid)
		}
	}
	for _, invalid := range []string{
		"customer-object://20/" + key,
		"customer-object://19/tenants/8/agents/build-a/inputs/a.apk",
		"customer-object://19/" + key + "?token=secret",
	} {
		if _, err := parseCustomerObjectReference(invalid, 19, prefix); err == nil {
			t.Fatalf("invalid object reference accepted: %s", invalid)
		}
	}
}

func TestCustomerTaskObjectKeyBindsAttemptAndType(t *testing.T) {
	prefix := "tenants/7/agents/build-a"
	if got := customerTaskObjectKey(prefix, 101, 3, core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILT_APK); got !=
		"tenants/7/agents/build-a/tasks/101/attempts/3/built.apk" {
		t.Fatalf("APK key=%q", got)
	}
	if got := customerTaskObjectKey(prefix, 101, 3, core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_BUILD_LOG); got !=
		"tenants/7/agents/build-a/tasks/101/attempts/3/build.log" {
		t.Fatalf("log key=%q", got)
	}
	if got := customerTaskObjectKey(prefix, 101, 3, core.HybridArtifactType_HYBRID_ARTIFACT_TYPE_SOURCE_APK); got != "" {
		t.Fatalf("unsupported key=%q", got)
	}
}
