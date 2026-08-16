package airgap

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
)

func taskFixture() TaskManifest {
	return TaskManifest{SchemaVersion: SchemaVersion, PackageCode: "package-12345678", Nonce: "nonce-12345678",
		TenantID: 7, AgentID: 19, AgentCertificateSerial: "abc", TaskID: 101, BuilderAttempt: 2,
		IssuedAt: 1000, ExpiresAt: 2000, Bundle: json.RawMessage(`{"schema_version":3}`), Inputs: []Artifact{{
			Role: "source_apk", Path: "inputs/00-source.apk", ObjectID: 1, ObjectType: 1,
			OriginalName: "source.apk", ContentType: "application/vnd.android.package-archive", SizeBytes: 3, SHA256: strings.Repeat("a", 64),
		}, {
			Role: "keystore", Path: "inputs/01-signing.jks", ObjectID: 2, ObjectType: 2,
			OriginalName: "signing.jks", ContentType: "application/octet-stream", SizeBytes: 4, SHA256: strings.Repeat("b", 64),
		}}}
}

func TestCanonicalSignedTaskEnvelope(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := taskFixture()
	canonical, err := CanonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := Sign(key, canonical)
	if err != nil {
		t.Fatal(err)
	}
	envelopeBytes, err := CanonicalJSON(TaskEnvelope{Manifest: manifest, Signature: signature})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeTaskEnvelope(envelopeBytes)
	if err != nil {
		t.Fatal(err)
	}
	decodedManifest, _ := CanonicalJSON(decoded.Manifest)
	if err := Verify(&key.PublicKey, decodedManifest, decoded.Signature); err != nil {
		t.Fatal(err)
	}

	tampered := append([]byte(nil), decodedManifest...)
	tampered[len(tampered)-2] ^= 1
	if err := Verify(&key.PublicKey, tampered, decoded.Signature); err == nil {
		t.Fatal("tampered AIR_GAPPED manifest signature was accepted")
	}
}

func TestDecodeRejectsUnknownNonCanonicalAndDuplicateArtifacts(t *testing.T) {
	manifest := taskFixture()
	envelope := TaskEnvelope{Manifest: manifest, Signature: Signature{Algorithm: SignatureAlgorithm, Value: "MAYCAQE="}}
	encoded, _ := CanonicalJSON(envelope)
	if _, err := DecodeTaskEnvelope(append(encoded, '\n')); err == nil {
		t.Fatal("non-canonical AIR_GAPPED JSON was accepted")
	}
	unknown := strings.Replace(string(encoded), `"signature":`, `"unknown":1,"signature":`, 1)
	if _, err := DecodeTaskEnvelope([]byte(unknown)); err == nil {
		t.Fatal("unknown AIR_GAPPED field was accepted")
	}
	manifest.Inputs = append(manifest.Inputs, manifest.Inputs[0])
	if err := ValidateTaskManifest(&manifest); err == nil {
		t.Fatal("duplicate AIR_GAPPED Artifact was accepted")
	}
}

func TestTaskManifestAllowsMultipleDistinctTemplateFiles(t *testing.T) {
	manifest := taskFixture()
	manifest.Inputs = append(manifest.Inputs,
		Artifact{Role: "template_file", Path: "inputs/templates/one.xml", ObjectID: 3, ObjectType: 7,
			OriginalName: "one.xml", ContentType: "application/xml", SizeBytes: 5, SHA256: strings.Repeat("c", 64)},
		Artifact{Role: "template_file", Path: "inputs/templates/two.png", ObjectID: 4, ObjectType: 7,
			OriginalName: "two.png", ContentType: "image/png", SizeBytes: 6, SHA256: strings.Repeat("d", 64)})
	if err := ValidateTaskManifest(&manifest); err != nil {
		t.Fatalf("distinct template files were rejected: %v", err)
	}
	canonical, err := CanonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeTaskManifest(canonical); err != nil {
		t.Fatalf("decode canonical task manifest: %v", err)
	}
}

func TestResultManifestRequiresSuccessAPKOrFailureMessage(t *testing.T) {
	base := ResultManifest{SchemaVersion: SchemaVersion, PackageCode: "package-12345678", Nonce: "nonce-12345678",
		TenantID: 7, AgentID: 19, AgentCertificateSerial: "abc", TaskID: 101, BuilderAttempt: 2,
		ExportPackageSHA256: strings.Repeat("a", 64), BuiltAt: 1500}
	base.Status = "SUCCESS"
	if err := ValidateResultManifest(&base); err == nil {
		t.Fatal("successful AIR_GAPPED result without APK was accepted")
	}
	base.Outputs = []Artifact{{Role: "built_apk", Path: "outputs/built.apk", OriginalName: "built.apk",
		ContentType: "application/vnd.android.package-archive", SizeBytes: 7, SHA256: strings.Repeat("b", 64)}}
	if err := ValidateResultManifest(&base); err != nil {
		t.Fatalf("valid successful AIR_GAPPED result was rejected: %v", err)
	}
	canonical, err := CanonicalJSON(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeResultManifest(canonical); err != nil {
		t.Fatalf("decode canonical result manifest: %v", err)
	}
	base.Status, base.ErrorMessage, base.Outputs = "FAILED", "LOCAL_EXECUTOR_FAILED", nil
	if err := ValidateResultManifest(&base); err != nil {
		t.Fatal(err)
	}
}
