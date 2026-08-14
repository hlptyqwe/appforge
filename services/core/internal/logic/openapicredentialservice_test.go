package logic

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"appforge/proto/core"
)

func TestGenerateOpenApiKey(t *testing.T) {
	t.Parallel()

	keyID, secret, fullKey, secretHash, err := generateOpenApiKey()
	if err != nil {
		t.Fatalf("generateOpenApiKey() error = %v", err)
	}
	if !strings.HasPrefix(fullKey, "afk_"+keyID+"_") || strings.Contains(secretHash, secret) {
		t.Fatalf("generated API key format or hash isolation is invalid")
	}
	digest := sha256.Sum256([]byte(secret))
	if secretHash != hex.EncodeToString(digest[:]) || !validSHA256(secretHash) {
		t.Fatalf("secret hash mismatch: %q", secretHash)
	}
	_, _, anotherKey, _, err := generateOpenApiKey()
	if err != nil || anotherKey == fullKey {
		t.Fatalf("API keys must be unique: first=%q second=%q err=%v", fullKey, anotherKey, err)
	}
}

func TestNormalizedOpenApiScopes(t *testing.T) {
	t.Parallel()

	values, encoded, err := normalizedOpenApiScopes([]core.OpenApiScope{
		core.OpenApiScope_OPEN_API_SCOPE_APPS_READ,
		core.OpenApiScope_OPEN_API_SCOPE_APPS_READ,
		core.OpenApiScope_OPEN_API_SCOPE_BUILDS_WRITE,
	})
	if err != nil {
		t.Fatalf("normalizedOpenApiScopes() error = %v", err)
	}
	if len(values) != 2 || encoded != `["apps:read","builds:write"]` {
		t.Fatalf("normalizedOpenApiScopes() = %v %s", values, encoded)
	}
	if _, _, err := normalizedOpenApiScopes([]core.OpenApiScope{core.OpenApiScope_OPEN_API_SCOPE_UNKNOWN}); err == nil {
		t.Fatal("normalizedOpenApiScopes() accepted unknown scope")
	}
}

func TestClientIPAllowed(t *testing.T) {
	t.Parallel()

	allowlist := []string{"203.0.113.8", "2001:db8::/32", "198.51.100.0/24"}
	for _, value := range []string{"203.0.113.8", "198.51.100.42", "2001:db8::9"} {
		if !clientIPAllowed(value, allowlist) {
			t.Fatalf("clientIPAllowed(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"203.0.113.9", "192.0.2.1", "invalid"} {
		if clientIPAllowed(value, allowlist) {
			t.Fatalf("clientIPAllowed(%q) = true, want false", value)
		}
	}
}
