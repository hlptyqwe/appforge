package logic

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"appforge/common/secretbox"
	"appforge/proto/core"
	"appforge/services/core/models"
)

func TestSourceIntegrationMappingNeverExposesTokens(t *testing.T) {
	item := &models.TSourceIntegration{
		Id: 9, TenantId: 3, Platform: int64(core.SourcePlatform_SOURCE_PLATFORM_GITHUB),
		IntegrationName: "GitHub", InstallationRef: "installation-1", AccessTokenCiphertext: "sb1.secret",
		RefreshTokenCiphertext: sql.NullString{String: "sb1.refresh", Valid: true}, Status: 1, CreateTime: time.Now(), UpdateTime: time.Now(),
	}
	mapped := mapSourceIntegration(item)
	if mapped.Id != item.Id || mapped.InstallationRef != item.InstallationRef {
		t.Fatalf("unexpected mapping: %+v", mapped)
	}
	encoded := mapped.String()
	if strings.Contains(encoded, "sb1.secret") || strings.Contains(encoded, "sb1.refresh") {
		t.Fatal("source integration mapping exposed encrypted token material")
	}
}

func TestSourceTokenEncryptionDoesNotPersistPlaintext(t *testing.T) {
	box, err := secretbox.New("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Seal("provider-token")
	if err != nil {
		t.Fatal(err)
	}
	if ciphertext == "provider-token" || strings.Contains(ciphertext, "provider-token") {
		t.Fatal("provider token was not encrypted")
	}
	plaintext, err := box.Open(ciphertext)
	if err != nil || plaintext != "provider-token" {
		t.Fatalf("token round trip failed: plaintext=%q err=%v", plaintext, err)
	}
}

func TestSourceInputValidators(t *testing.T) {
	if !validSourcePlatform(core.SourcePlatform_SOURCE_PLATFORM_GITHUB) || !validSourcePlatform(core.SourcePlatform_SOURCE_PLATFORM_GITLAB) {
		t.Fatal("supported provider was rejected")
	}
	if validSourcePlatform(core.SourcePlatform_SOURCE_PLATFORM_UNKNOWN) {
		t.Fatal("unknown provider was accepted")
	}
	for _, value := range []string{strings.Repeat("a", 40), strings.Repeat("F", 64), "abcdef0"} {
		if !isHexLength(value, 7, 64) {
			t.Fatalf("valid hexadecimal identifier rejected: %q", value)
		}
	}
	for _, value := range []string{"xyz1234", "abc", strings.Repeat("a", 65)} {
		if isHexLength(value, 7, 64) {
			t.Fatalf("invalid hexadecimal identifier accepted: %q", value)
		}
	}
}
