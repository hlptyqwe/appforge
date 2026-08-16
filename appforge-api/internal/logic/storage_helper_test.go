package logic

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"appforge/proto/core"
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

func TestVerifyBrandingImageBoundaries(t *testing.T) {
	logo := writeBrandingPNG(t, 512, 512)
	if err := verifyBrandingImage(logo, core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO); err != nil {
		t.Fatalf("valid logo rejected: %v", err)
	}
	splash := writeBrandingPNG(t, 720, 1280)
	if err := verifyBrandingImage(splash, core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH); err != nil {
		t.Fatalf("valid splash rejected: %v", err)
	}
	for name, test := range map[string]struct {
		filename   string
		objectType core.StorageObjectType
	}{
		"non-square logo": {writeBrandingPNG(t, 512, 513), core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO},
		"small logo":      {writeBrandingPNG(t, 511, 511), core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO},
		"large logo":      {writeBrandingPNG(t, 2049, 2049), core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO},
		"small splash":    {writeBrandingPNG(t, 719, 1280), core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH},
		"fake image":      {writeBrandingBytes(t, []byte("not-an-image")), core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO},
		"animated webp":   {writeBrandingBytes(t, brandingVP8X(true, 512, 512)), core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO},
	} {
		t.Run(name, func(t *testing.T) {
			if err := verifyBrandingImage(test.filename, test.objectType); err == nil {
				t.Fatal("invalid branding image was accepted")
			}
		})
	}
	webp := writeBrandingBytes(t, brandingVP8X(false, 512, 512))
	if err := verifyBrandingImage(webp, core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO); err != nil {
		t.Fatalf("valid static WebP header rejected: %v", err)
	}
}

func writeBrandingPNG(t *testing.T, width, height int) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "image.png")
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	value := image.NewNRGBA(image.Rect(0, 0, width, height))
	value.Set(0, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 255})
	encodeErr := png.Encode(file, value)
	closeErr := file.Close()
	if encodeErr != nil || closeErr != nil {
		t.Fatalf("write PNG: encode=%v close=%v", encodeErr, closeErr)
	}
	return filename
}

func writeBrandingBytes(t *testing.T, content []byte) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "image.webp")
	if err := os.WriteFile(filename, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return filename
}

func brandingVP8X(animated bool, width, height int) []byte {
	content := make([]byte, 30)
	copy(content[0:4], "RIFF")
	binary.LittleEndian.PutUint32(content[4:8], uint32(len(content)-8))
	copy(content[8:12], "WEBP")
	copy(content[12:16], "VP8X")
	binary.LittleEndian.PutUint32(content[16:20], 10)
	if animated {
		content[20] = 0x02
	}
	width--
	height--
	content[24], content[25], content[26] = byte(width), byte(width>>8), byte(width>>16)
	content[27], content[28], content[29] = byte(height), byte(height>>8), byte(height>>16)
	return content
}
