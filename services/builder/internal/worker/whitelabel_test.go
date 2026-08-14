package worker

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"appforge/common/secretbox"
)

func TestApplyWhiteLabelTemplate(t *testing.T) {
	decodedDir := t.TempDir()
	manifestPath := filepath.Join(decodedDir, "AndroidManifest.xml")
	valuesDir := filepath.Join(decodedDir, "res", "values")
	xmlDir := filepath.Join(decodedDir, "res", "xml")
	rawDir := filepath.Join(decodedDir, "res", "raw")
	if err := os.MkdirAll(valuesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(xmlDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rawDir, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.source.app"><application android:name=".SmokeApplication" android:label="@string/app_name"><provider android:name="com.external.Provider" android:authorities="source.fileprovider"/><activity android:name=".MainActivity"><intent-filter><data android:scheme="sourceoauth"/><data android:scheme="https" android:host="source.example.com"/></intent-filter></activity></application></manifest>`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(valuesDir, "strings.xml"), []byte(`<resources><string name="oauth_scheme">sourceoauth</string></resources>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xmlDir, "provider.xml"), []byte(`<paths name="com.source.app.files"/>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rawDir, "customer.json"), []byte(`{"old":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	templateFile := filepath.Join(t.TempDir(), "customer.json")
	if err := os.WriteFile(templateFile, []byte(`{"customer":"isolated"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	snapshot := &whiteLabelBuildSnapshot{
		ProductID: 1, TemplateID: 2, TemplateRevision: 3,
		TemplateChecksum: strings.Repeat("a", 64), CertificateSHA256: strings.Repeat("b", 64),
		OriginalPackageName: "com.source.app", TargetPackageName: "com.customer.product",
		ParameterValuesJSON: `{"oauthScheme":"customoauth","firebase":{"project_id":"demo"}}`,
		ManifestPatchJSON:   `[{"op":"manifest.setPackage"},{"op":"manifest.replaceProviderAuthority","old":"source.fileprovider","new":"{{packageName}}.provider"},{"op":"manifest.replaceIntentScheme","old":"sourceoauth","new":"{{parameters.oauthScheme}}"},{"op":"manifest.replaceAppLinkHost","old":"source.example.com","new":"customer.example.com"}]`,
		ResourcePatchJSON:   `[{"op":"resource.replaceString","name":"oauth_scheme","valueParameter":"oauthScheme"},{"op":"asset.writeJson","path":"assets/google-services.json","contentParameter":"firebase"},{"op":"resource.replaceFile","path":"res/raw/customer.json","objectId":17}]`,
		ExtensionFilesJSON:  `[]`,
		parameters:          map[string]any{"oauthScheme": "customoauth", "firebase": map[string]any{"project_id": "demo"}},
	}
	if err := applyWhiteLabelTemplate(decodedDir, manifestPath, snapshot, map[int64]string{17: templateFile}); err != nil {
		t.Fatalf("apply template failed: %v", err)
	}
	updatedManifest, _ := os.ReadFile(manifestPath)
	manifestText := string(updatedManifest)
	if !strings.Contains(manifestText, `package="com.customer.product"`) ||
		!strings.Contains(manifestText, `android:name="com.source.app.SmokeApplication"`) ||
		!strings.Contains(manifestText, `android:name="com.source.app.MainActivity"`) ||
		!strings.Contains(manifestText, `android:name="com.external.Provider"`) ||
		!strings.Contains(manifestText, "customoauth") ||
		!strings.Contains(manifestText, "com.customer.product.provider") ||
		!strings.Contains(manifestText, "customer.example.com") {
		t.Fatalf("unexpected manifest: %s", updatedManifest)
	}
	updatedStrings, _ := os.ReadFile(filepath.Join(valuesDir, "strings.xml"))
	if !strings.Contains(string(updatedStrings), "customoauth") {
		t.Fatalf("string resource was not replaced: %s", updatedStrings)
	}
	googleServices, err := os.ReadFile(filepath.Join(decodedDir, "assets", "google-services.json"))
	if err != nil || !strings.Contains(string(googleServices), `"project_id":"demo"`) {
		t.Fatalf("controlled extension was not written: %s err=%v", googleServices, err)
	}
	replacedFile, err := os.ReadFile(filepath.Join(rawDir, "customer.json"))
	if err != nil || string(replacedFile) != `{"customer":"isolated"}` {
		t.Fatalf("resource file was not replaced: %s err=%v", replacedFile, err)
	}
}

func TestProtectManifestComponentNamesAcrossPackageRewrite(t *testing.T) {
	manifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.source"><application android:name="App"><activity android:name=".Main"/><activity-alias android:name=".Alias" android:targetActivity=".Main"/><service android:name="com.source.Sync"/><provider android:name="com.external.Provider" android:authorities="com.source.files"/></application></manifest>`
	protected, componentNames, err := protectManifestComponentNames(manifest, "com.source")
	if err != nil {
		t.Fatal(err)
	}
	rewritten := strings.ReplaceAll(protected, "com.source", "com.customer")
	restored := restoreManifestComponentNames(rewritten, componentNames)
	for _, expected := range []string{
		`android:name="com.source.App"`,
		`android:name="com.source.Main"`,
		`android:name="com.source.Alias"`,
		`android:targetActivity="com.source.Main"`,
		`android:name="com.source.Sync"`,
		`android:name="com.external.Provider"`,
		`android:authorities="com.customer.files"`,
	} {
		if !strings.Contains(restored, expected) {
			t.Fatalf("expected %q in restored manifest: %s", expected, restored)
		}
	}
}

func TestDecodeWhiteLabelBuildSnapshotRejectsIncompleteSnapshot(t *testing.T) {
	if _, err := decodeWhiteLabelBuildSnapshot(`{"productId":1}`); err == nil {
		t.Fatal("expected incomplete snapshot to be rejected")
	}
}

func TestDecryptSensitiveWhiteLabelParameters(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	box, err := secretbox.New(key)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal("oauth-client-secret")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &whiteLabelBuildSnapshot{parameters: map[string]any{
		"secret": sealed,
		"nested": map[string]any{"plain": "visible"},
	}}
	if err := snapshot.decryptSensitiveParameters(box.Open); err != nil {
		t.Fatalf("decrypt sensitive parameters failed: %v", err)
	}
	if snapshot.parameters["secret"] != "oauth-client-secret" {
		t.Fatalf("unexpected decrypted value: %#v", snapshot.parameters)
	}
}
