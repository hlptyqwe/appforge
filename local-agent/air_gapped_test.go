package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"appforge/common/airgap"
)

func TestDecodeAirGappedBuildBundleBindsPackagedInputs(t *testing.T) {
	payload := []byte("synthetic-apk")
	artifact := airgap.Artifact{Role: "source_apk", Path: "inputs/source.apk", ObjectID: 71, ObjectType: 1,
		OriginalName: "source.apk", ContentType: "application/vnd.android.package-archive", SizeBytes: int64(len(payload)),
		SHA256: airgap.Digest(payload)}
	task := &task{ID: 31, TenantID: 7, AppID: 9, BuilderAttempt: 2}
	bundle := buildManifest{SchemaVersion: protocolVersion, Task: task, PackageName: "com.example.synthetic", KeyAlias: "release",
		SigningSecretRef: "local-file:///signing.json", SignerCertificateSHA256: strings.Repeat("a", 64), Inputs: []buildInput{{
			Role: artifact.Role, ObjectID: artifact.ObjectID, ObjectType: artifact.ObjectType, OriginalName: artifact.OriginalName,
			ContentType: artifact.ContentType, SizeBytes: artifact.SizeBytes, SHA256: artifact.SHA256,
		}}}
	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	manifest := &airgap.TaskManifest{TaskID: task.ID, TenantID: task.TenantID, BuilderAttempt: task.BuilderAttempt,
		Bundle: raw, Inputs: []airgap.Artifact{artifact}}
	inputPath := filepath.Join(t.TempDir(), "source.apk")
	decoded, err := decodeAirGappedBuildBundle(manifest, map[string]string{artifact.Path: inputPath})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Inputs[0].LocalPath != inputPath || decoded.SigningSecretRef != bundle.SigningSecretRef {
		t.Fatalf("unexpected decoded bundle: %#v", decoded)
	}
	manifest.Inputs[0].SHA256 = strings.Repeat("b", 64)
	if _, err := decodeAirGappedBuildBundle(manifest, map[string]string{artifact.Path: inputPath}); err == nil {
		t.Fatal("bundle and package integrity mismatch was accepted")
	}
}

func TestAirGappedReplayMarkerRejectsSecondExecution(t *testing.T) {
	stateDir := t.TempDir()
	manifest := &airgap.TaskManifest{PackageCode: "agp_replay01", Nonce: "agn_replay01"}
	lock, err := lockAirGappedReplay(stateDir, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.consume(airGappedReplayMarker{PackageCode: manifest.PackageCode, NonceSHA256: airgap.Digest([]byte(manifest.Nonce)),
		ExportPackageSHA256: strings.Repeat("a", 64), ResultPackageSHA256: strings.Repeat("b", 64), ConsumedAt: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	lock.close()
	if _, err := lockAirGappedReplay(stateDir, manifest); err == nil || !strings.Contains(err.Error(), "already consumed") {
		t.Fatalf("consumed task was not rejected: %v", err)
	}
}

func TestAirGappedResultIsSignedAndPublishedWithoutOverwrite(t *testing.T) {
	_, certificate, privateKey := airGappedCertificateFixture(t, 7, 13)
	workDir := t.TempDir()
	apkPath := filepath.Join(workDir, "channel.apk")
	logPath := filepath.Join(workDir, "build.log")
	apk, logData := []byte("synthetic-built-apk"), []byte("synthetic build log\n")
	if err := os.WriteFile(apkPath, apk, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, logData, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := &airgap.TaskManifest{PackageCode: "agp_result01", Nonce: "agn_result01", TenantID: 7, AgentID: 13,
		TaskID: 19, BuilderAttempt: 2, IssuedAt: time.Now().Add(-time.Minute).UnixMilli(), ExpiresAt: time.Now().Add(time.Hour).UnixMilli()}
	envelope, paths, err := createAirGappedResultEnvelope(manifest, certificate, privateKey, strings.Repeat("a", 64), workDir,
		&buildResult{APKPath: apkPath, APKSize: int64(len(apk)), APKSHA256: airgap.Digest(apk),
			LogPath: logPath, LogSize: int64(len(logData)), LogSHA256: airgap.Digest(logData)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "result.zip")
	digest, err := publishAirGappedResult(resultPath, envelope, paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(digest) != 64 {
		t.Fatalf("result digest=%q", digest)
	}
	file, err := os.Open(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := file.Stat()
	decoded, err := airgap.ReadResultPackage(file, info.Size(), nil)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := airgap.CanonicalJSON(decoded.Manifest)
	if err := airgap.Verify(&privateKey.PublicKey, canonical, decoded.Signature); err != nil {
		t.Fatal(err)
	}
	if decoded.Manifest.Status != "SUCCESS" || len(decoded.Manifest.Outputs) != 2 {
		t.Fatalf("unexpected result manifest: %#v", decoded.Manifest)
	}
	if _, err := publishAirGappedResult(resultPath, envelope, paths); err == nil {
		t.Fatal("existing result package was overwritten")
	}
}

func TestVerifyAirGappedTaskIdentityChecksCASignature(t *testing.T) {
	ca, certificate, privateKey := airGappedCertificateFixture(t, 7, 13)
	root := t.TempDir()
	certPath, keyPath, caPath := filepath.Join(root, "client.crt"), filepath.Join(root, "client.key"), filepath.Join(root, "agent-ca.crt")
	writeCertificateFixture(t, certPath, certificate)
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	writeCertificateFixture(t, caPath, ca.certificate)
	manifest := airgap.TaskManifest{SchemaVersion: airgap.SchemaVersion, PackageCode: "agp_verify01", Nonce: "agn_verify01",
		TenantID: 7, AgentID: 13, AgentCertificateSerial: strings.ToLower(certificate.SerialNumber.Text(16)), TaskID: 19,
		BuilderAttempt: 2, IssuedAt: time.Now().Add(-time.Minute).UnixMilli(), ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
		Bundle: json.RawMessage(`{"schema_version":3}`), Inputs: []airgap.Artifact{{Role: "source_apk", Path: "inputs/source.apk",
			OriginalName: "source.apk", ContentType: "application/vnd.android.package-archive", SizeBytes: 1, SHA256: strings.Repeat("a", 64)},
			{Role: "keystore", Path: "inputs/signing.jks", OriginalName: "signing.jks", ContentType: "application/octet-stream",
				SizeBytes: 1, SHA256: strings.Repeat("b", 64)}}}
	canonical, _ := airgap.CanonicalJSON(manifest)
	signature, err := airgap.Sign(ca.privateKey, canonical)
	if err != nil {
		t.Fatal(err)
	}
	state := &state{AgentID: 13, Certificate: certPath, PrivateKey: keyPath, ClientCA: caPath}
	if _, _, err := verifyAirGappedTaskIdentity(state, &airgap.TaskEnvelope{Manifest: manifest, Signature: signature}); err != nil {
		t.Fatal(err)
	}
	signature.Value = strings.Repeat("A", len(signature.Value))
	if _, _, err := verifyAirGappedTaskIdentity(state, &airgap.TaskEnvelope{Manifest: manifest, Signature: signature}); err == nil {
		t.Fatal("invalid control-plane signature was accepted")
	}
}

type airGappedCAFixture struct {
	certificate *x509.Certificate
	privateKey  *ecdsa.PrivateKey
}

func airGappedCertificateFixture(t *testing.T, tenantID, agentID int64) (*airGappedCAFixture, *x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Synthetic Agent CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCertificate, _ := x509.ParseCertificate(caDER)
	agentKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	identity, _ := url.Parse("spiffe://appforge/tenant/" + big.NewInt(tenantID).String() + "/local-agent/" + big.NewInt(agentID).String())
	agentTemplate := &x509.Certificate{SerialNumber: big.NewInt(13), Subject: pkix.Name{CommonName: "Synthetic Agent"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, URIs: []*url.URL{identity}}
	agentDER, err := x509.CreateCertificate(rand.Reader, agentTemplate, caCertificate, &agentKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	agentCertificate, _ := x509.ParseCertificate(agentDER)
	return &airGappedCAFixture{certificate: caCertificate, privateKey: caKey}, agentCertificate, agentKey
}

func writeCertificateFixture(t *testing.T, path string, certificate *x509.Certificate) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
}
