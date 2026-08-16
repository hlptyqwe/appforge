package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"appforge/proto/core"
)

func TestDecodeBuildBrandingSnapshotStrictIdentity(t *testing.T) {
	value := `{"profileId":9,"revision":3,"appName":"AppForge","logoObjectId":1,"splashObjectId":2,"apiHost":"https://api.example.com","rewriteMode":1,"launcherIconTarget":"mipmap/ic_launcher","splashResourceTarget":"drawable/splash_logo"}`
	snapshot, err := decodeBuildBrandingSnapshot(value)
	if err != nil || snapshot.ProfileID != 9 || snapshot.Revision != 3 || snapshot.AppName != "AppForge" {
		t.Fatalf("valid branding snapshot rejected: snapshot=%+v err=%v", snapshot, err)
	}
	for _, invalid := range []string{
		`{}`, `{"profileId":9,"revision":0,"appName":"AppForge","launcherIconTarget":"mipmap/ic_launcher"}`,
		`{"profileId":9,"revision":1,"appName":"","launcherIconTarget":"mipmap/ic_launcher"}`,
		`{"profileId":9,"revision":1,"appName":"AppForge","launcherIconTarget":"../icon"}`,
	} {
		if _, err := decodeBuildBrandingSnapshot(invalid); err == nil {
			t.Fatalf("invalid branding snapshot accepted: %s", invalid)
		}
	}
}

func TestBrandingPreflightReturnsLocatableDecodeFailure(t *testing.T) {
	report, _ := inspectBrandingCompatibility(context.Background(), filepath.Join(t.TempDir(), "missing.apk"), t.TempDir(), &core.BrandingProfile{})
	if report.Compatible || len(report.Checks) != 1 || report.Checks[0].Name != "apktool_decode" || report.Checks[0].Passed ||
		strings.TrimSpace(report.Checks[0].Message) == "" {
		t.Fatalf("unexpected incompatible preflight report: %+v", report)
	}
}

func TestBrandingPreflightResourceInspection(t *testing.T) {
	root := t.TempDir()
	values := filepath.Join(root, "res", "values")
	icons := filepath.Join(root, "res", "mipmap-hdpi")
	if err := os.MkdirAll(values, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(icons, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "strings.xml"), []byte(`<resources><string name="app_name">Source</string></resources>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(icons, "ic_launcher.png"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ok, _ := inspectLabelResource(root, `<application android:label="@string/app_name"/>`); !ok {
		t.Fatal("existing label resource was not detected")
	}
	if ok, _ := inspectRasterResource(root, "mipmap/ic_launcher"); !ok {
		t.Fatal("existing raster target was not detected")
	}
	if ok, message := inspectRasterResource(root, "drawable/missing"); ok || !strings.Contains(message, "未找到") {
		t.Fatalf("missing raster target did not return a locatable reason: ok=%t message=%q", ok, message)
	}
}
