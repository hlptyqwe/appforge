package handler

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"testing"

	"appforge/proto/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDecodeAgentRPCBodyRejectsUnknownFields(t *testing.T) {
	var request core.HeartbeatLocalAgentReq
	if err := decodeAgentRPCBody([]byte(`{"command":"sh"}`), &request); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unknown Agent RPC field code=%v err=%v", status.Code(err), err)
	}
}

func TestVerifiedAgentIdentityAcceptsPresentedCertificateChain(t *testing.T) {
	leaf := &x509.Certificate{SerialNumber: big.NewInt(42), Raw: []byte("leaf-certificate")}
	intermediate := &x509.Certificate{SerialNumber: big.NewInt(43), Raw: []byte("intermediate-certificate")}
	request := &http.Request{TLS: &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf, intermediate}}}
	identity, err := verifiedAgentIdentity(request)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(leaf.Raw)
	if identity.serial != "2a" || identity.fingerprint != hex.EncodeToString(digest[:]) {
		t.Fatalf("identity derived from wrong certificate: %#v", identity)
	}
}

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

func TestPrepareCustomerStorageBundleBindsAgentAndPrefix(t *testing.T) {
	execution := localAgentExecutionFixture()
	for _, object := range []*core.StorageObject{execution.SourceApk, execution.Keystore} {
		object.StorageMode = core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE
		object.OwnerAgentId = 19
		object.ObjectKey = "tenants/7/agents/build-a/inputs/apps/11/" + strings.TrimSuffix(object.OriginalName, ".apk")
	}
	bundle, err := buildLocalAgentManifest(execution)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareCustomerStorageBundle(bundle,
		"local-file:///customer-storage.json#tenants/7/agents/build-a", 19); err != nil {
		t.Fatal(err)
	}
	for _, input := range bundle.Inputs {
		if !strings.HasPrefix(input.CustomerReference, "customer-object://19/tenants/7/agents/build-a/") {
			t.Fatalf("unexpected customer reference %q", input.CustomerReference)
		}
		if input.DownloadPath != "" {
			t.Fatal("customer input unexpectedly received a control-plane download ticket")
		}
	}
}

func TestPrepareCustomerStorageBundleRejectsWrongOwnerAndPrefix(t *testing.T) {
	bundle := &localAgentBuildManifest{Task: &core.BuildTask{Id: 101}, Inputs: []localAgentBuildInput{{
		Role: "source_apk", StorageMode: core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE,
		OwnerAgentID: 20, objectKey: "tenants/7/agents/build-a/inputs/apps/11/1/source.apk",
	}}}
	reference := "local-file:///customer-storage.json#tenants/7/agents/build-a"
	if code := status.Code(prepareCustomerStorageBundle(bundle, reference, 19)); code != codes.PermissionDenied {
		t.Fatalf("wrong owner code=%v", code)
	}
	bundle.Inputs[0].OwnerAgentID = 19
	bundle.Inputs[0].objectKey = "tenants/8/agents/build-a/inputs/apps/11/1/source.apk"
	if code := status.Code(prepareCustomerStorageBundle(bundle, reference, 19)); code != codes.PermissionDenied {
		t.Fatalf("outside prefix code=%v", code)
	}
}

func TestControlPlaneBundleRejectsCustomerObject(t *testing.T) {
	bundle := &localAgentBuildManifest{Inputs: []localAgentBuildInput{{
		Role: "keystore", StorageMode: core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CUSTOMER_STORAGE, OwnerAgentID: 19,
	}}}
	if code := status.Code(validateControlPlaneBundleInputs(bundle)); code != codes.FailedPrecondition {
		t.Fatalf("customer object in control-plane bundle code=%v", code)
	}
}

func localAgentExecutionFixture() *core.BuildExecutionContext {
	task := &core.BuildTask{Id: 101, TenantId: 7, AppId: 11, BuilderAttempt: 2}
	object := func(id int64, objectType core.StorageObjectType, name, digest string) *core.StorageObject {
		return &core.StorageObject{Id: id, TenantId: task.TenantId, AppId: task.AppId, ObjectType: objectType,
			ObjectKey: "tenants/7/test/" + name, OriginalName: name, ContentType: "application/octet-stream", SizeBytes: 128, Sha256: strings.Repeat(digest, 64),
			Status:      core.StorageObjectStatus_STORAGE_OBJECT_STATUS_BOUND,
			StorageMode: core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE}
	}
	return &core.BuildExecutionContext{Task: task, PackageName: "com.example.app", ApiHost: "https://api.example.com",
		ChannelName: "website", KeyAlias: "release", SignerCertificateSha256: strings.Repeat("c", 64),
		SourceApk: object(1, core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK, "source.apk", "a"),
		Keystore:  object(2, core.StorageObjectType_STORAGE_OBJECT_TYPE_KEYSTORE, "release.jks", "b")}
}
