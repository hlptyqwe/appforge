package logic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyTemplateFileRejectsPlaintextSecret(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "oauth.json")
	if err := os.WriteFile(filename, []byte(`{"client_id":"demo","client_secret":"plaintext"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyTemplateFile(filename, "oauth.json"); err == nil || !strings.Contains(err.Error(), "plaintext secrets") {
		t.Fatalf("expected plaintext secret rejection, err=%v", err)
	}
}

func TestVerifyTemplateFileAllowsFirebaseAPIKey(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "google-services.json")
	if err := os.WriteFile(filename, []byte(`{"project_info":{"project_id":"demo"},"client":[{"api_key":[{"current_key":"firebase-public-api-key"}]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyTemplateFile(filename, "google-services.json"); err != nil {
		t.Fatalf("expected Firebase API key configuration to be allowed: %v", err)
	}
}
