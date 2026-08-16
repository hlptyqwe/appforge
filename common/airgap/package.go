package airgap

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"
)

const (
	MaxPackageBytes   int64 = 3 * 1024 * 1024 * 1024
	MaxPackageEntries       = 260
)

var zipEpoch = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// ArtifactOpener opens one declared Artifact. The caller retains ownership of
// external resources; the returned stream is always closed by the package writer.
type ArtifactOpener func(artifact Artifact) (io.ReadCloser, error)

// ArtifactConsumer receives a verified, size-limited stream. It must consume
// the whole stream before returning.
type ArtifactConsumer func(artifact Artifact, reader io.Reader) error

// WriteTaskPackage writes a deterministic, uncompressed task ZIP and verifies
// every source stream while copying it.
func WriteTaskPackage(output io.Writer, envelope TaskEnvelope, open ArtifactOpener) error {
	manifest, err := CanonicalJSON(envelope)
	if err != nil {
		return err
	}
	if _, err := DecodeTaskEnvelope(manifest); err != nil {
		return err
	}
	return writePackage(output, TaskManifestName, manifest, envelope.Manifest.Inputs, open)
}

// WriteResultPackage writes the corresponding Agent-signed result ZIP.
func WriteResultPackage(output io.Writer, envelope ResultEnvelope, open ArtifactOpener) error {
	manifest, err := CanonicalJSON(envelope)
	if err != nil {
		return err
	}
	if _, err := DecodeResultEnvelope(manifest); err != nil {
		return err
	}
	return writePackage(output, ResultManifestName, manifest, envelope.Manifest.Outputs, open)
}

func writePackage(output io.Writer, manifestName string, manifest []byte, artifacts []Artifact, open ArtifactOpener) error {
	if output == nil || open == nil || len(artifacts)+1 > MaxPackageEntries {
		return errors.New("AIR_GAPPED package writer input is invalid")
	}
	writer := zip.NewWriter(output)
	closed := false
	defer func() {
		if !closed {
			_ = writer.Close()
		}
	}()
	if err := writeZipBytes(writer, manifestName, manifest); err != nil {
		return err
	}
	for _, artifact := range artifacts {
		if err := validatePackagePath(artifact.Path); err != nil {
			return err
		}
		header := &zip.FileHeader{Name: artifact.Path, Method: zip.Store, UncompressedSize64: uint64(artifact.SizeBytes)}
		header.SetMode(0o600)
		header.SetModTime(zipEpoch)
		target, err := writer.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("create AIR_GAPPED ZIP entry: %w", err)
		}
		source, err := open(artifact)
		if err != nil {
			return fmt.Errorf("open AIR_GAPPED Artifact %q: %w", artifact.Role, err)
		}
		hasher := sha256.New()
		written, copyErr := io.Copy(io.MultiWriter(target, hasher), io.LimitReader(source, artifact.SizeBytes+1))
		closeErr := source.Close()
		if copyErr != nil {
			return fmt.Errorf("copy AIR_GAPPED Artifact %q: %w", artifact.Role, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close AIR_GAPPED Artifact %q: %w", artifact.Role, closeErr)
		}
		if written != artifact.SizeBytes || hex.EncodeToString(hasher.Sum(nil)) != artifact.SHA256 {
			return fmt.Errorf("AIR_GAPPED Artifact %q changed while packaging", artifact.Role)
		}
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close AIR_GAPPED ZIP: %w", err)
	}
	closed = true
	return nil
}

func writeZipBytes(writer *zip.Writer, name string, data []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Store, UncompressedSize64: uint64(len(data))}
	header.SetMode(0o600)
	header.SetModTime(zipEpoch)
	target, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = target.Write(data)
	return err
}

// ReadTaskPackage strictly validates and consumes a task package.
func ReadTaskPackage(reader io.ReaderAt, size int64, consume ArtifactConsumer) (*TaskEnvelope, error) {
	archive, entries, err := openPackage(reader, size, TaskManifestName)
	if err != nil {
		return nil, err
	}
	manifest, err := readManifest(entries[TaskManifestName])
	if err != nil {
		return nil, err
	}
	envelope, err := DecodeTaskEnvelope(manifest)
	if err != nil {
		return nil, err
	}
	if err := consumePackageArtifacts(archive, entries, TaskManifestName, envelope.Manifest.Inputs, consume); err != nil {
		return nil, err
	}
	return envelope, nil
}

// ReadResultPackage strictly validates and consumes a result package.
func ReadResultPackage(reader io.ReaderAt, size int64, consume ArtifactConsumer) (*ResultEnvelope, error) {
	archive, entries, err := openPackage(reader, size, ResultManifestName)
	if err != nil {
		return nil, err
	}
	manifest, err := readManifest(entries[ResultManifestName])
	if err != nil {
		return nil, err
	}
	envelope, err := DecodeResultEnvelope(manifest)
	if err != nil {
		return nil, err
	}
	if err := consumePackageArtifacts(archive, entries, ResultManifestName, envelope.Manifest.Outputs, consume); err != nil {
		return nil, err
	}
	return envelope, nil
}

func openPackage(reader io.ReaderAt, size int64, manifestName string) (*zip.Reader, map[string]*zip.File, error) {
	if reader == nil || size <= 0 || size > MaxPackageBytes {
		return nil, nil, errors.New("AIR_GAPPED package size is invalid")
	}
	archive, err := zip.NewReader(reader, size)
	if err != nil || len(archive.File) == 0 || len(archive.File) > MaxPackageEntries {
		return nil, nil, errors.New("AIR_GAPPED ZIP structure is invalid")
	}
	entries := make(map[string]*zip.File, len(archive.File))
	for _, entry := range archive.File {
		if err := validatePackagePath(entry.Name); err != nil || entry.Method != zip.Store || !entry.Mode().IsRegular() ||
			entry.UncompressedSize64 > uint64(MaxPackageBytes) || entry.CompressedSize64 != entry.UncompressedSize64 ||
			entry.Comment != "" {
			return nil, nil, errors.New("AIR_GAPPED ZIP entry is unsafe")
		}
		if _, exists := entries[entry.Name]; exists {
			return nil, nil, errors.New("AIR_GAPPED ZIP contains duplicate entries")
		}
		entries[entry.Name] = entry
	}
	if entries[manifestName] == nil {
		return nil, nil, errors.New("AIR_GAPPED ZIP manifest is missing")
	}
	return archive, entries, nil
}

func readManifest(entry *zip.File) ([]byte, error) {
	if entry == nil || entry.UncompressedSize64 == 0 || entry.UncompressedSize64 > MaxManifestBytes {
		return nil, errors.New("AIR_GAPPED ZIP manifest size is invalid")
	}
	reader, err := entry.Open()
	if err != nil {
		return nil, errors.New("AIR_GAPPED ZIP manifest cannot be opened")
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, MaxManifestBytes+1))
	if err != nil || uint64(len(data)) != entry.UncompressedSize64 {
		return nil, errors.New("AIR_GAPPED ZIP manifest cannot be read")
	}
	return data, nil
}

func consumePackageArtifacts(_ *zip.Reader, entries map[string]*zip.File, manifestName string, artifacts []Artifact, consume ArtifactConsumer) error {
	expected := make(map[string]Artifact, len(artifacts))
	total := int64(0)
	for _, artifact := range artifacts {
		if _, exists := expected[artifact.Path]; exists {
			return errors.New("AIR_GAPPED manifest contains duplicate paths")
		}
		expected[artifact.Path] = artifact
		total += artifact.SizeBytes
		if total < 0 || total > MaxPackageBytes {
			return errors.New("AIR_GAPPED declared Artifact bytes exceed the package limit")
		}
	}
	if len(entries) != len(expected)+1 {
		return errors.New("AIR_GAPPED ZIP contains unknown or missing files")
	}
	paths := make([]string, 0, len(expected))
	for artifactPath := range expected {
		paths = append(paths, artifactPath)
	}
	sort.Strings(paths)
	for _, artifactPath := range paths {
		artifact := expected[artifactPath]
		entry := entries[artifactPath]
		if entry == nil || entry.Name == manifestName || entry.UncompressedSize64 != uint64(artifact.SizeBytes) {
			return fmt.Errorf("AIR_GAPPED Artifact %q entry metadata mismatch", artifact.Role)
		}
		reader, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open AIR_GAPPED Artifact %q: %w", artifact.Role, err)
		}
		hasher := sha256.New()
		limited := io.LimitReader(reader, artifact.SizeBytes+1)
		if consume != nil {
			err = consume(artifact, io.TeeReader(limited, hasher))
		} else {
			_, err = io.Copy(hasher, limited)
		}
		if err == nil {
			var extra [1]byte
			if count, readErr := limited.Read(extra[:]); count != 0 || (readErr != nil && readErr != io.EOF) {
				err = errors.New("Artifact consumer did not consume exact bytes")
			}
		}
		closeErr := reader.Close()
		if err != nil {
			return fmt.Errorf("consume AIR_GAPPED Artifact %q: %w", artifact.Role, err)
		}
		if closeErr != nil {
			return fmt.Errorf("close AIR_GAPPED Artifact %q: %w", artifact.Role, closeErr)
		}
		if hex.EncodeToString(hasher.Sum(nil)) != artifact.SHA256 {
			return fmt.Errorf("AIR_GAPPED Artifact %q SHA-256 mismatch", artifact.Role)
		}
	}
	return nil
}

func validatePackagePath(value string) error {
	if value == "" || strings.TrimSpace(value) != value || strings.Contains(value, "\\") || strings.ContainsAny(value, "\x00\r\n") ||
		strings.HasPrefix(value, "/") || path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") || strings.Contains(value, "/../") {
		return errors.New("AIR_GAPPED package path is unsafe")
	}
	return nil
}
