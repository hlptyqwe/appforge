package worker

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"appforge/proto/core"
)

const brandingAssetPath = "assets/appforge/branding.json"

type buildBrandingSnapshot struct {
	ProfileID            int64  `json:"profileId"`
	Revision             int64  `json:"revision"`
	AppName              string `json:"appName"`
	LogoObjectID         int64  `json:"logoObjectId"`
	SplashObjectID       int64  `json:"splashObjectId"`
	APIHost              string `json:"apiHost"`
	RewriteMode          int64  `json:"rewriteMode"`
	LauncherIconTarget   string `json:"launcherIconTarget"`
	SplashResourceTarget string `json:"splashResourceTarget"`
	RuntimeConfigJSON    string `json:"runtimeConfig"`
}

type brandingAssetPayload struct {
	SchemaVersion int             `json:"schemaVersion"`
	ProfileID     int64           `json:"profileId"`
	Revision      int64           `json:"revision"`
	AppName       string          `json:"appName"`
	APIHost       string          `json:"apiHost"`
	LogoSHA256    string          `json:"logoSha256"`
	SplashSHA256  string          `json:"splashSha256"`
	RuntimeConfig json.RawMessage `json:"runtimeConfig,omitempty"`
}

func decodeBuildBrandingSnapshot(value string) (*buildBrandingSnapshot, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var snapshot buildBrandingSnapshot
	if err := json.Unmarshal([]byte(value), &snapshot); err != nil {
		return nil, fmt.Errorf("decode branding snapshot: %w", err)
	}
	if snapshot.ProfileID <= 0 || snapshot.Revision <= 0 || strings.TrimSpace(snapshot.AppName) == "" ||
		!resourceTargetPattern.MatchString(strings.TrimPrefix(snapshot.LauncherIconTarget, "@")) {
		return nil, fmt.Errorf("branding snapshot is incomplete")
	}
	return &snapshot, nil
}

func buildBrandedAPK(ctx context.Context, logFile io.Writer, sourcePath, logoPath, splashPath, workDir string,
	snapshot *buildBrandingSnapshot, whiteLabel *whiteLabelBuildSnapshot, templateFiles map[int64]string,
	logo, splash *core.StorageObject, channel channelPayload) (string, error) {
	decodedDir := filepath.Join(workDir, "decoded")
	if err := runAndLog(ctx, logFile, nil, "apktool", "d", "-f", "-o", decodedDir, sourcePath); err != nil {
		return "", fmt.Errorf("decode APK for branding: %w", err)
	}
	manifestPath := filepath.Join(decodedDir, "AndroidManifest.xml")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", fmt.Errorf("read decoded manifest: %w", err)
	}
	if whiteLabel != nil {
		if err := applyWhiteLabelTemplate(decodedDir, manifestPath, whiteLabel, templateFiles); err != nil {
			return "", err
		}
		manifest, err = os.ReadFile(manifestPath)
		if err != nil {
			return "", fmt.Errorf("read white-label manifest: %w", err)
		}
	}
	if err := replaceApplicationLabel(decodedDir, manifestPath, manifest, snapshot.AppName); err != nil {
		return "", err
	}
	iconTarget := strings.TrimPrefix(snapshot.LauncherIconTarget, "@")
	if err := replaceRasterResources(ctx, logFile, decodedDir, iconTarget, logoPath, true); err != nil {
		return "", fmt.Errorf("replace launcher icon: %w", err)
	}
	if snapshot.RewriteMode == int64(core.BrandingRewriteMode_BRANDING_REWRITE_MODE_RESOURCE_REBUILD) {
		if err := replaceRasterResources(ctx, logFile, decodedDir, strings.TrimPrefix(snapshot.SplashResourceTarget, "@"), splashPath, false); err != nil {
			return "", fmt.Errorf("replace splash resource: %w", err)
		}
	}
	assetsDir := filepath.Join(decodedDir, "assets", "appforge")
	if err := os.MkdirAll(assetsDir, 0o700); err != nil {
		return "", fmt.Errorf("create AppForge assets: %w", err)
	}
	runtimeConfig := json.RawMessage(nil)
	if value := strings.TrimSpace(snapshot.RuntimeConfigJSON); value != "" {
		if !json.Valid([]byte(value)) {
			return "", fmt.Errorf("branding runtime config is invalid")
		}
		runtimeConfig = json.RawMessage(value)
	}
	brandingPayload := brandingAssetPayload{SchemaVersion: 1, ProfileID: snapshot.ProfileID, Revision: snapshot.Revision,
		AppName: snapshot.AppName, APIHost: snapshot.APIHost, LogoSHA256: logo.GetSha256(),
		SplashSHA256: splash.GetSha256(), RuntimeConfig: runtimeConfig}
	if err := writeJSONFile(filepath.Join(decodedDir, brandingAssetPath), brandingPayload); err != nil {
		return "", err
	}
	if err := writeJSONFile(filepath.Join(decodedDir, channelAssetPath), channel); err != nil {
		return "", err
	}
	if whiteLabel != nil {
		if err := writeJSONFile(filepath.Join(decodedDir, whiteLabelAssetPath), whiteLabel.publicAsset()); err != nil {
			return "", err
		}
	}
	unsignedPath := filepath.Join(workDir, "unsigned.apk")
	if err := runAndLog(ctx, logFile, nil, "apktool", "b", decodedDir, "-o", unsignedPath); err != nil {
		return "", fmt.Errorf("rebuild branded APK: %w", err)
	}
	return unsignedPath, nil
}

func replaceApplicationLabel(decodedDir, manifestPath string, manifest []byte, appName string) error {
	match := manifestLabelPattern.FindSubmatch(manifest)
	if len(match) != 2 {
		return fmt.Errorf("application label is unavailable")
	}
	label := string(match[1])
	if !strings.HasPrefix(label, "@string/") {
		escaped := new(strings.Builder)
		if err := xml.EscapeText(escaped, []byte(appName)); err != nil {
			return err
		}
		updated := manifestLabelPattern.ReplaceAllStringFunc(string(manifest), func(string) string {
			return `android:label="` + escaped.String() + `"`
		})
		return os.WriteFile(manifestPath, []byte(updated), 0o600)
	}
	resourceName := strings.TrimPrefix(label, "@string/")
	if !regexp.MustCompile(`^[a-z0-9_.]+$`).MatchString(resourceName) {
		return fmt.Errorf("application label resource name is unsafe")
	}
	files, _ := filepath.Glob(filepath.Join(decodedDir, "res", "values*", "*.xml"))
	pattern := regexp.MustCompile(`(?s)(<string\s+[^>]*name="` + regexp.QuoteMeta(resourceName) + `"[^>]*>).*?(</string>)`)
	escaped := new(strings.Builder)
	if err := xml.EscapeText(escaped, []byte(appName)); err != nil {
		return err
	}
	replaced := 0
	for _, filename := range files {
		content, err := os.ReadFile(filename)
		if err != nil || !pattern.Match(content) {
			continue
		}
		updated := pattern.ReplaceAllStringFunc(string(content), func(match string) string {
			start := strings.Index(match, ">")
			end := strings.LastIndex(match, "</string>")
			if start < 0 || end < start {
				return match
			}
			return match[:start+1] + escaped.String() + match[end:]
		})
		if err := os.WriteFile(filename, []byte(updated), 0o600); err != nil {
			return err
		}
		replaced++
	}
	if replaced == 0 {
		return fmt.Errorf("application label resource %q was not found", resourceName)
	}
	return nil
}

func replaceRasterResources(ctx context.Context, output io.Writer, decodedDir, target, source string, launcher bool) error {
	if !resourceTargetPattern.MatchString(target) {
		return fmt.Errorf("resource target is unsafe or empty")
	}
	parts := strings.SplitN(target, "/", 2)
	files, _ := filepath.Glob(filepath.Join(decodedDir, "res", parts[0]+"*", parts[1]+".*"))
	replaced := 0
	for _, filename := range files {
		ext := strings.ToLower(filepath.Ext(filename))
		if ext != ".png" && ext != ".webp" {
			continue
		}
		args := []string{source}
		if launcher {
			if size := launcherSize(filepath.Base(filepath.Dir(filename))); size > 0 {
				args = append(args, "-resize", fmt.Sprintf("%dx%d!", size, size))
			}
		}
		args = append(args, filename)
		if err := runAndLog(ctx, output, nil, "convert", args...); err != nil {
			return err
		}
		replaced++
	}
	if replaced == 0 {
		return fmt.Errorf("no PNG/WebP resource matched %q", target)
	}
	return nil
}

func launcherSize(directory string) int {
	switch {
	case strings.Contains(directory, "xxxhdpi"):
		return 192
	case strings.Contains(directory, "xxhdpi"):
		return 144
	case strings.Contains(directory, "xhdpi"):
		return 96
	case strings.Contains(directory, "hdpi"):
		return 72
	case strings.Contains(directory, "mdpi"):
		return 48
	default:
		return 0
	}
}

func writeJSONFile(filename string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filename, encoded, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(filename), err)
	}
	return nil
}
