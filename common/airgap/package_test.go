package airgap

import (
	"archive/zip"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"os"
	"strings"
	"testing"
)

func signedTaskPackageFixture(t *testing.T) (TaskEnvelope, map[string][]byte) {
	t.Helper()
	files := map[string][]byte{
		"inputs/source.apk":  []byte("synthetic-apk"),
		"inputs/signing.jks": []byte("synthetic-keystore"),
	}
	manifest := TaskManifest{SchemaVersion: SchemaVersion, PackageCode: "package-12345678", Nonce: "nonce-12345678",
		TenantID: 7, AgentID: 19, AgentCertificateSerial: "abc", TaskID: 101, BuilderAttempt: 2,
		IssuedAt: 1000, ExpiresAt: 2000, Bundle: []byte(`{"schema_version":3}`), Inputs: []Artifact{
			{Role: "source_apk", Path: "inputs/source.apk", ObjectID: 1, ObjectType: 1, OriginalName: "source.apk",
				ContentType: "application/vnd.android.package-archive", SizeBytes: int64(len(files["inputs/source.apk"])), SHA256: Digest(files["inputs/source.apk"])},
			{Role: "keystore", Path: "inputs/signing.jks", ObjectID: 2, ObjectType: 2, OriginalName: "signing.jks",
				ContentType: "application/octet-stream", SizeBytes: int64(len(files["inputs/signing.jks"])), SHA256: Digest(files["inputs/signing.jks"])},
		}}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := CanonicalJSON(manifest)
	signature, err := Sign(key, canonical)
	if err != nil {
		t.Fatal(err)
	}
	return TaskEnvelope{Manifest: manifest, Signature: signature}, files
}

func TestTaskPackageRoundTripAndTamperRejection(t *testing.T) {
	envelope, files := signedTaskPackageFixture(t)
	var encoded bytes.Buffer
	err := WriteTaskPackage(&encoded, envelope, func(artifact Artifact) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(files[artifact.Path])), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	consumed := map[string][]byte{}
	decoded, err := ReadTaskPackage(bytes.NewReader(encoded.Bytes()), int64(encoded.Len()), func(artifact Artifact, reader io.Reader) error {
		data, readErr := io.ReadAll(reader)
		consumed[artifact.Path] = data
		return readErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Manifest.PackageCode != envelope.Manifest.PackageCode || !bytes.Equal(consumed["inputs/source.apk"], files["inputs/source.apk"]) {
		t.Fatal("AIR_GAPPED task package round trip changed data")
	}

	tampered := append([]byte(nil), encoded.Bytes()...)
	index := bytes.Index(tampered, files["inputs/source.apk"])
	if index < 0 {
		t.Fatal("source bytes not found in Store ZIP")
	}
	tampered[index] ^= 1
	if _, err := ReadTaskPackage(bytes.NewReader(tampered), int64(len(tampered)), nil); err == nil {
		t.Fatal("tampered AIR_GAPPED task package was accepted")
	}
}

func TestPackageWriterRejectsChangedArtifact(t *testing.T) {
	envelope, files := signedTaskPackageFixture(t)
	files["inputs/source.apk"] = []byte("changed")
	if err := WriteTaskPackage(io.Discard, envelope, func(artifact Artifact) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(files[artifact.Path])), nil
	}); err == nil {
		t.Fatal("changed source Artifact was accepted")
	}
}

func TestPackageReaderRejectsUnknownAndSymlinkEntries(t *testing.T) {
	envelope, files := signedTaskPackageFixture(t)
	manifest, _ := CanonicalJSON(envelope)
	for _, test := range []struct {
		name    string
		unknown bool
		symlink bool
	}{
		{name: "unknown", unknown: true},
		{name: "symlink", symlink: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var encoded bytes.Buffer
			writer := zip.NewWriter(&encoded)
			writeRawZipEntry(t, writer, TaskManifestName, manifest, 0o600)
			for path, data := range files {
				mode := os.FileMode(0o600)
				if test.symlink && strings.Contains(path, "source") {
					mode = os.ModeSymlink | 0o777
				}
				writeRawZipEntry(t, writer, path, data, mode)
			}
			if test.unknown {
				writeRawZipEntry(t, writer, "inputs/unknown.bin", []byte("unknown"), 0o600)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadTaskPackage(bytes.NewReader(encoded.Bytes()), int64(encoded.Len()), nil); err == nil {
				t.Fatalf("unsafe %s package was accepted", test.name)
			}
		})
	}
}

func writeRawZipEntry(t *testing.T, writer *zip.Writer, name string, data []byte, mode os.FileMode) {
	t.Helper()
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(mode)
	header.SetModTime(zipEpoch)
	target, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.Write(data); err != nil {
		t.Fatal(err)
	}
}
