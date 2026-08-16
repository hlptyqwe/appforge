package logic

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"appforge/proto/core"
	"appforge/services/core/models"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidateBrandingFieldsSecurityBoundaries(t *testing.T) {
	valid := func(apiHost, launcher, splash, runtime string) error {
		return validateBrandingFields("default", "AppForge", apiHost,
			core.BrandingRewriteMode_BRANDING_REWRITE_MODE_RESOURCE_REBUILD, launcher, splash, runtime)
	}
	for _, apiHost := range []string{"https://api.example.com", "http://localhost:8080", "http://127.0.0.1:8080"} {
		if err := valid(apiHost, "mipmap/ic_launcher", "drawable/splash_logo", `{}`); err != nil {
			t.Fatalf("valid API host %q rejected: %v", apiHost, err)
		}
	}
	for name, apiHost := range map[string]string{
		"plaintext remote": "http://api.example.com",
		"credentials":      "https://user:pass@api.example.com",
		"path":             "https://api.example.com/v1",
		"query":            "https://api.example.com?token=value",
	} {
		t.Run(name, func(t *testing.T) {
			if err := valid(apiHost, "mipmap/ic_launcher", "drawable/splash_logo", `{}`); err == nil {
				t.Fatalf("unsafe API host %q accepted", apiHost)
			}
		})
	}
	if err := valid("https://api.example.com", "../mipmap/icon", "drawable/splash_logo", `{}`); err == nil {
		t.Fatal("unsafe launcher resource accepted")
	}
	if err := valid("https://api.example.com", "mipmap/ic_launcher", "drawable/splash-logo", `{}`); err == nil {
		t.Fatal("unsafe splash resource accepted")
	}
	if err := valid("https://api.example.com", "mipmap/ic_launcher", "drawable/splash_logo", `{broken`); err == nil {
		t.Fatal("invalid runtime JSON accepted")
	}
}

func TestValidateBrandingStorageObjectInput(t *testing.T) {
	valid := &core.CreateStorageObjectReq{
		AppId: 9, ObjectType: core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO,
		ObjectKey: "tenants/7/brand-logo/fixture.png", OriginalName: "fixture.png", SizeBytes: 5 * 1024 * 1024,
	}
	if err := validateStorageObjectInput(valid, 7); err != nil {
		t.Fatalf("valid branding object rejected: %v", err)
	}
	tests := []struct {
		name       string
		objectType core.StorageObjectType
		key        string
		filename   string
		size       int64
	}{
		{name: "cross tenant", objectType: core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO, key: "tenants/8/brand-logo/a.png", filename: "a.png", size: 1},
		{name: "path traversal", objectType: core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO, key: "tenants/7/../a.png", filename: "a.png", size: 1},
		{name: "wrong extension", objectType: core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO, key: "tenants/7/brand-logo/a.svg", filename: "a.svg", size: 1},
		{name: "oversized logo", objectType: core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO, key: "tenants/7/brand-logo/a.png", filename: "a.png", size: 5*1024*1024 + 1},
		{name: "oversized splash", objectType: core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH, key: "tenants/7/brand-splash/a.webp", filename: "a.webp", size: 10*1024*1024 + 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := &core.CreateStorageObjectReq{AppId: 9, ObjectType: test.objectType, ObjectKey: test.key, OriginalName: test.filename, SizeBytes: test.size}
			if err := validateStorageObjectInput(input, 7); err == nil {
				t.Fatal("invalid branding object metadata accepted")
			}
		})
	}
}

func TestParseBrandingSnapshotRequiresIdentityAndRevision(t *testing.T) {
	valid := `{"profileId":9,"revision":3,"appName":"AppForge","logoObjectId":1,"splashObjectId":2,"apiHost":"https://api.example.com","rewriteMode":1}`
	snapshot, err := parseBrandingSnapshot(valid)
	if err != nil || snapshot.ProfileID != 9 || snapshot.Revision != 3 {
		t.Fatalf("valid snapshot rejected: snapshot=%+v err=%v", snapshot, err)
	}
	for _, input := range []string{`{}`, `{"profileId":9}`, `{"profileId":9,"revision":0}`, strings.Repeat("x", 10)} {
		if _, err := parseBrandingSnapshot(input); err == nil {
			t.Fatalf("invalid snapshot accepted: %s", input)
		}
	}
}

func TestBrandingPreflightNodeEligible(t *testing.T) {
	now := time.Now()
	healthy := models.TBuilderNode{
		Status:               builderNodeStatusOnline,
		DrainStatus:          builderDrainAccepting,
		MaxConcurrency:       2,
		RunningCount:         0,
		DiskCapacity:         100 * 1024 * 1024 * 1024,
		DiskFree:             10 * 1024 * 1024 * 1024,
		ToolchainVersion:     "android-debian-v4",
		BuildProtocolVersion: 1,
		CapabilityJson:       sql.NullString{String: `{"apk":true,"branding":true}`, Valid: true},
		LastHeartbeatAt:      now.Add(-time.Second),
	}
	eligible, err := brandingPreflightNodeEligible(&healthy, now)
	if err != nil || !eligible {
		t.Fatalf("healthy branding node rejected: eligible=%t err=%v", eligible, err)
	}

	for _, test := range []struct {
		name   string
		mutate func(*models.TBuilderNode)
	}{
		{name: "isolated", mutate: func(node *models.TBuilderNode) { node.Status = builderNodeStatusIsolated }},
		{name: "draining", mutate: func(node *models.TBuilderNode) { node.DrainStatus = builderDrainDraining }},
		{name: "stale heartbeat", mutate: func(node *models.TBuilderNode) { node.LastHeartbeatAt = now.Add(-2 * time.Minute) }},
		{name: "future heartbeat", mutate: func(node *models.TBuilderNode) { node.LastHeartbeatAt = now.Add(time.Second) }},
		{name: "full", mutate: func(node *models.TBuilderNode) { node.RunningCount = node.MaxConcurrency }},
		{name: "less than 512 MiB", mutate: func(node *models.TBuilderNode) { node.DiskFree = builderMinimumDiskFree - 1 }},
		{name: "less than two percent", mutate: func(node *models.TBuilderNode) {
			node.DiskCapacity = 100 * 1024 * 1024 * 1024
			node.DiskFree = 1024 * 1024 * 1024
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			node := healthy
			test.mutate(&node)
			eligible, err := brandingPreflightNodeEligible(&node, now)
			if err != nil || eligible {
				t.Fatalf("unavailable node accepted: eligible=%t err=%v", eligible, err)
			}
		})
	}

	for _, test := range []struct {
		name   string
		mutate func(*models.TBuilderNode)
	}{
		{name: "missing toolchain", mutate: func(node *models.TBuilderNode) { node.ToolchainVersion = "" }},
		{name: "invalid capability", mutate: func(node *models.TBuilderNode) { node.CapabilityJson = sql.NullString{String: `{`, Valid: true} }},
		{name: "branding capability disabled", mutate: func(node *models.TBuilderNode) {
			node.CapabilityJson = sql.NullString{String: `{"apk":true,"branding":false}`, Valid: true}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			node := healthy
			test.mutate(&node)
			eligible, err := brandingPreflightNodeEligible(&node, now)
			if eligible || status.Code(err) != codes.FailedPrecondition {
				t.Fatalf("invalid capability result: eligible=%t code=%v err=%v", eligible, status.Code(err), err)
			}
		})
	}
}
