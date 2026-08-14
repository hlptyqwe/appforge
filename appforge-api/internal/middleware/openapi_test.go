package middleware

import (
	"context"
	"strings"
	"testing"

	"appforge/proto/core"
)

func TestParseOpenApiKey(t *testing.T) {
	secret := strings.Repeat("a", 32)
	keyID, parsedSecret, ok := parseOpenApiKey("Bearer afk_testKey_" + secret)
	if !ok || keyID != "testKey" || parsedSecret != secret {
		t.Fatalf("parseOpenApiKey() = (%q, %q, %v)", keyID, parsedSecret, ok)
	}
	invalid := []string{
		"",
		"Basic afk_testKey_" + secret,
		"Bearer invalid",
		"Bearer afk__" + secret,
		"Bearer afk_testKey_short",
	}
	for _, value := range invalid {
		if _, _, valid := parseOpenApiKey(value); valid {
			t.Fatalf("parseOpenApiKey(%q) unexpectedly succeeded", value)
		}
	}
}

func TestParseGeneratedOpenApiKeyAllowsBase64URLUnderscores(t *testing.T) {
	keyID := "RZVujIv6Pj_L"
	secret := "ab_cdEFGhijklmnopqrstuvwxyz0123456789ABCDEF"
	if len(secret) != 43 {
		t.Fatalf("test secret length = %d", len(secret))
	}
	parsedKeyID, parsedSecret, ok := parseOpenApiKey("Bearer afk_" + keyID + "_" + secret)
	if !ok || parsedKeyID != keyID || parsedSecret != secret {
		t.Fatalf("parseOpenApiKey() = (%q, %q, %v)", parsedKeyID, parsedSecret, ok)
	}
}

func TestRequireOpenApiScopeAndApp(t *testing.T) {
	principal := &OpenApiPrincipal{
		Scopes: map[core.OpenApiScope]struct{}{
			core.OpenApiScope_OPEN_API_SCOPE_APPS_READ: {},
		},
		AppIDs: map[int64]struct{}{7: {}},
	}
	ctx := context.WithValue(context.Background(), openApiPrincipalKey{}, principal)
	if !RequireOpenApiScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_APPS_READ, 7) {
		t.Fatal("authorized scope and application were rejected")
	}
	if RequireOpenApiScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_APPS_WRITE, 7) {
		t.Fatal("missing scope was accepted")
	}
	if RequireOpenApiScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_APPS_READ, 8) {
		t.Fatal("application outside allowlist was accepted")
	}
}

func TestOpenApiRateLimit(t *testing.T) {
	middleware := NewOpenApiMiddleware(nil)
	remaining, _, allowed := middleware.allow(9, 2)
	if !allowed || remaining != 1 {
		t.Fatalf("first request = (%d, %v), want (1, true)", remaining, allowed)
	}
	remaining, _, allowed = middleware.allow(9, 2)
	if !allowed || remaining != 0 {
		t.Fatalf("second request = (%d, %v), want (0, true)", remaining, allowed)
	}
	remaining, _, allowed = middleware.allow(9, 2)
	if allowed || remaining != 0 {
		t.Fatalf("third request = (%d, %v), want (0, false)", remaining, allowed)
	}
}
