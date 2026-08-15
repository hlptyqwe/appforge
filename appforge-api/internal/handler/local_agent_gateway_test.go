package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"appforge/proto/core"
)

func TestBuildLocalAgentManifestUsesOnlyWhitelistedSigningReference(t *testing.T) {
	execution := localAgentExecutionFixture()
	execution.SecretRef = "local-file:///signing/app.json"
	execution.KeystorePasswordCiphertext = "sb1.control-plane-store-password"
	execution.KeyPasswordCiphertext = "sb1.control-plane-key-password"
	bundle, err := buildLocalAgentManifest(execution)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.BlockedReason != "" || bundle.SigningSecretRef != execution.SecretRef {
		t.Fatalf("unexpected bundle: %#v", bundle)
	}
	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)
	if strings.Contains(encoded, "control-plane-store-password") || strings.Contains(encoded, "control-plane-key-password") {
		t.Fatal("control-plane signing ciphertext leaked into Local Agent manifest")
	}
}

func TestBuildLocalAgentManifestBlocksControlPlaneSecrets(t *testing.T) {
	execution := localAgentExecutionFixture()
	execution.TemplateSnapshotJson = `{"parameterValuesJson":"{\"clientSecret\":\"sb1.encrypted\"}"}`
	bundle, err := buildLocalAgentManifest(execution)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.BlockedReason != "LOCAL_SIGNING_SECRET_REQUIRED" {
		t.Fatalf("blocked reason = %q", bundle.BlockedReason)
	}
	if bundle.TemplateSnapshotJSON != "" {
		t.Fatal("encrypted template snapshot was exposed")
	}
}

func TestBuildLocalAgentManifestRejectsCrossTenantInput(t *testing.T) {
	execution := localAgentExecutionFixture()
	execution.SecretRef = "local-file:///signing/app.json"
	execution.Keystore.TenantId = 99
	if _, err := buildLocalAgentManifest(execution); err == nil {
		t.Fatal("cross-tenant Keystore input was accepted")
	}
}

func localAgentExecutionFixture() *core.BuildExecutionContext {
	task := &core.BuildTask{Id: 101, TenantId: 7, AppId: 11, BuilderAttempt: 2}
	object := func(id int64, objectType core.StorageObjectType, name, digest string) *core.StorageObject {
		return &core.StorageObject{Id: id, TenantId: task.TenantId, AppId: task.AppId, ObjectType: objectType,
			OriginalName: name, ContentType: "application/octet-stream", SizeBytes: 128, Sha256: strings.Repeat(digest, 64),
			Status: core.StorageObjectStatus_STORAGE_OBJECT_STATUS_BOUND}
	}
	return &core.BuildExecutionContext{Task: task, PackageName: "com.example.app", ApiHost: "https://api.example.com",
		ChannelName: "website", KeyAlias: "release", SignerCertificateSha256: strings.Repeat("c", 64),
		SourceApk: object(1, core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK, "source.apk", "a"),
		Keystore:  object(2, core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE, "release.jks", "b")}
}
