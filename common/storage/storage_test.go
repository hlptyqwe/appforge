package storage

import (
	"strings"
	"testing"
)

func TestGenerateTenantObjectKeyKeepsTenantNamespaceAndExtension(t *testing.T) {
	key, err := GenerateTenantObjectKey(42, "source-apk", "Release.APK")
	if err != nil {
		t.Fatalf("GenerateTenantObjectKey() error = %v", err)
	}
	if !strings.HasPrefix(key, "tenants/42/source-apk/") {
		t.Fatalf("key %q is outside tenant namespace", key)
	}
	if !strings.HasSuffix(key, ".apk") {
		t.Fatalf("key %q does not preserve normalized extension", key)
	}
}

func TestGenerateTenantObjectKeyRejectsInvalidTenant(t *testing.T) {
	if _, err := GenerateTenantObjectKey(0, "source-apk", "release.apk"); err == nil {
		t.Fatal("GenerateTenantObjectKey() expected tenant validation error")
	}
}
