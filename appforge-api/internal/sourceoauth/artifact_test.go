package sourceoauth

import (
	"archive/zip"
	"os"
	"strings"
	"testing"

	"appforge/admin-api/internal/config"
)

func TestExtractSingleAPK(t *testing.T) {
	archive := writeTestArchive(t, map[string]string{"notes.txt": "ignored", "outputs/app-release.apk": "apk-bytes"})
	extracted, err := extractSingleAPK(archive)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(extracted) })
	data, err := os.ReadFile(extracted)
	if err != nil || string(data) != "apk-bytes" {
		t.Fatalf("unexpected extracted APK: data=%q err=%v", data, err)
	}
}

func TestExtractSingleAPKRejectsAmbiguousArchive(t *testing.T) {
	archive := writeTestArchive(t, map[string]string{"one.apk": "one", "two.apk": "two"})
	if _, err := extractSingleAPK(archive); err == nil {
		t.Fatal("archive containing multiple APK files was accepted")
	}
}

func TestProviderRedirectAllowlist(t *testing.T) {
	provider := config.SourceOAuthProviderConfig{ApiBaseURL: "https://api.github.com"}
	if target, err := resolveProviderRedirect("https://api.github.com/a", "https://objects.githubusercontent.com/signed", provider); err != nil || target == "" {
		t.Fatalf("trusted provider redirect was rejected: %v", err)
	}
	for _, target := range []string{"http://objects.githubusercontent.com/file", "https://127.0.0.1/file", "https://attacker.example/file"} {
		if _, err := resolveProviderRedirect("https://api.github.com/a", target, provider); err == nil {
			t.Fatalf("unsafe redirect was accepted: %s", target)
		}
	}
}

func writeTestArchive(t *testing.T, files map[string]string) string {
	t.Helper()
	file, err := os.CreateTemp("", "appforge-artifact-test-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(file.Name()) })
	writer := zip.NewWriter(file)
	for name, value := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(value)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(file.Name())
}
