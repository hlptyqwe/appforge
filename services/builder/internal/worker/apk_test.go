package worker

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectChannelAssetReplacesPayloadAndRemovesSignatures(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.apk")
	outputPath := filepath.Join(dir, "output.apk")
	file, err := os.Create(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, value := range map[string]string{
		"AndroidManifest.xml":          "manifest",
		"classes.dex":                  "dex",
		"META-INF/CERT.RSA":            "signature",
		"assets/appforge/channel.json": "old",
	} {
		entry, createErr := writer.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := entry.Write([]byte(value)); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	err = injectChannelAsset(sourcePath, outputPath, channelPayload{SchemaVersion: 1, ChannelCode: "google", VersionID: 12, BuildTaskID: 34})
	if err != nil {
		t.Fatalf("injectChannelAsset() error = %v", err)
	}
	result, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	var payload string
	for _, entry := range result.File {
		if strings.HasPrefix(strings.ToUpper(entry.Name), "META-INF/") {
			t.Fatalf("signature entry was preserved: %s", entry.Name)
		}
		if entry.Name == channelAssetPath {
			reader, openErr := entry.Open()
			if openErr != nil {
				t.Fatal(openErr)
			}
			content, readErr := io.ReadAll(reader)
			_ = reader.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			payload = string(content)
		}
	}
	if !strings.Contains(payload, `"channelCode":"google"`) {
		t.Fatalf("unexpected channel payload: %s", payload)
	}
	if !strings.Contains(payload, `"versionId":12`) || !strings.Contains(payload, `"buildTaskId":34`) {
		t.Fatalf("task snapshot is missing from channel payload: %s", payload)
	}
}
