package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"appforge/proto/core"
)

var (
	manifestLabelPattern  = regexp.MustCompile(`android:label="([^"]+)"`)
	manifestIconPattern   = regexp.MustCompile(`android:icon="@([a-z0-9_]+)/([a-z0-9_.]+)"`)
	resourceTargetPattern = regexp.MustCompile(`^[a-z0-9_]+/[a-z0-9_.]+$`)
)

type preflightCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

type preflightReport struct {
	SchemaVersion int              `json:"schemaVersion"`
	Compatible    bool             `json:"compatible"`
	Toolchain     string           `json:"toolchain"`
	Checks        []preflightCheck `json:"checks"`
}

func (w *Worker) claimAndExecutePreflight(parent context.Context) (bool, error) {
	claimCtx, cancel := context.WithTimeout(parent, 15*time.Second)
	response, err := w.core.ClaimBrandingPreflight(claimCtx, &core.ClaimBrandingPreflightReq{
		BuilderId: w.config.Builder.Id, LeaseSeconds: 600,
	})
	cancel()
	if err != nil {
		return false, err
	}
	if response.GetData() == nil || response.Data.GetPreflight().GetId() <= 0 {
		return false, nil
	}
	data := response.Data
	preflight := data.Preflight
	workDir, err := os.MkdirTemp(w.config.Builder.TempDir, fmt.Sprintf("preflight-%d-", preflight.Id))
	if err != nil {
		return true, w.completePreflight(parent, preflight, core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_FAILED,
			preflightReport{SchemaVersion: 1, Checks: []preflightCheck{{Name: "workspace", Passed: false, Message: err.Error()}}}, "unknown")
	}
	defer os.RemoveAll(workDir)
	sourcePath := filepath.Join(workDir, "source.apk")
	if err := w.downloadVerified(parent, data.SourceApk, sourcePath); err != nil {
		return true, w.completePreflight(parent, preflight, core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_FAILED,
			preflightReport{SchemaVersion: 1, Checks: []preflightCheck{{Name: "source_apk", Passed: false, Message: err.Error()}}}, "unknown")
	}
	report, toolchain := inspectBrandingCompatibility(parent, sourcePath, workDir, data.Profile)
	state := core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_INCOMPATIBLE
	if report.Compatible {
		state = core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_COMPATIBLE
	}
	return true, w.completePreflight(parent, preflight, state, report, toolchain)
}

func (w *Worker) completePreflight(ctx context.Context, item *core.BrandingPreflight, state core.BrandingPreflightStatus, report preflightReport, toolchain string) error {
	report.Compatible = state == core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_COMPATIBLE
	report.Toolchain = toolchain
	encoded, err := json.Marshal(report)
	if err != nil {
		return err
	}
	_, err = w.core.CompleteBrandingPreflight(context.WithoutCancel(ctx), &core.CompleteBrandingPreflightReq{
		Id: item.Id, Status: state, ReportJson: string(encoded), ToolchainVersion: toolchain,
		BuilderId: w.config.Builder.Id, BuilderAttempt: item.BuilderAttempt,
	})
	return err
}

func inspectBrandingCompatibility(ctx context.Context, sourcePath, workDir string, profile *core.BrandingProfile) (preflightReport, string) {
	report := preflightReport{SchemaVersion: 1, Checks: make([]preflightCheck, 0, 6)}
	toolchain := commandVersion(ctx, "apktool", "--version")
	decodedDir := filepath.Join(workDir, "decoded")
	if output, err := exec.CommandContext(ctx, "apktool", "d", "-f", "-o", decodedDir, sourcePath).CombinedOutput(); err != nil {
		report.Checks = append(report.Checks, preflightCheck{Name: "apktool_decode", Message: commandFailure(err, output)})
		return report, toolchain
	}
	report.Checks = append(report.Checks, preflightCheck{Name: "apktool_decode", Passed: true, Message: "APK解码成功"})
	manifest, err := os.ReadFile(filepath.Join(decodedDir, "AndroidManifest.xml"))
	if err != nil {
		report.Checks = append(report.Checks, preflightCheck{Name: "manifest", Message: "无法读取解码后的AndroidManifest.xml"})
		return report, toolchain
	}
	labelOK, labelMessage := inspectLabelResource(decodedDir, string(manifest))
	report.Checks = append(report.Checks, preflightCheck{Name: "application_label", Passed: labelOK, Message: labelMessage})
	iconTarget := strings.TrimSpace(profile.GetLauncherIconTarget())
	if iconTarget == "" {
		match := manifestIconPattern.FindStringSubmatch(string(manifest))
		if len(match) == 3 {
			iconTarget = match[1] + "/" + match[2]
		}
	}
	iconOK, iconMessage := inspectRasterResource(decodedDir, iconTarget)
	report.Checks = append(report.Checks, preflightCheck{Name: "launcher_icon", Passed: iconOK, Message: iconMessage})
	splashOK, splashMessage := true, "Runtime Contract提供启动图"
	if profile.GetRewriteMode() == core.BrandingRewriteMode_BRANDING_REWRITE_MODE_RESOURCE_REBUILD {
		splashOK, splashMessage = inspectRasterResource(decodedDir, profile.GetSplashResourceTarget())
	}
	report.Checks = append(report.Checks, preflightCheck{Name: "splash_resource", Passed: splashOK, Message: splashMessage})
	rebuiltPath := filepath.Join(workDir, "preflight-rebuilt.apk")
	rebuildOK := true
	rebuildMessage := "未修改APK重建成功"
	if output, err := exec.CommandContext(ctx, "apktool", "b", decodedDir, "-o", rebuiltPath).CombinedOutput(); err != nil {
		rebuildOK = false
		rebuildMessage = commandFailure(err, output)
	}
	report.Checks = append(report.Checks, preflightCheck{Name: "apktool_rebuild", Passed: rebuildOK, Message: rebuildMessage})
	report.Compatible = labelOK && iconOK && splashOK && rebuildOK
	return report, toolchain
}

func inspectLabelResource(decodedDir, manifest string) (bool, string) {
	match := manifestLabelPattern.FindStringSubmatch(manifest)
	if len(match) != 2 {
		return false, "application label不存在"
	}
	if !strings.HasPrefix(match[1], "@string/") {
		return true, "application label为可替换字符串"
	}
	name := strings.TrimPrefix(match[1], "@string/")
	files, _ := filepath.Glob(filepath.Join(decodedDir, "res", "values*", "*.xml"))
	needle := `name="` + name + `"`
	for _, filename := range files {
		content, err := os.ReadFile(filename)
		if err == nil && strings.Contains(string(content), needle) {
			return true, "application label资源可解析"
		}
	}
	return false, "application label资源不存在: " + name
}

func inspectRasterResource(decodedDir, target string) (bool, string) {
	target = strings.TrimPrefix(strings.TrimSpace(target), "@")
	if !resourceTargetPattern.MatchString(target) {
		return false, "资源目标为空或格式无效"
	}
	parts := strings.SplitN(target, "/", 2)
	files, _ := filepath.Glob(filepath.Join(decodedDir, "res", parts[0]+"*", parts[1]+".*"))
	for _, filename := range files {
		ext := strings.ToLower(filepath.Ext(filename))
		if ext == ".png" || ext == ".webp" {
			return true, "找到可替换位图资源: " + target
		}
	}
	return false, "未找到PNG/WebP位图资源: " + target
}

func commandVersion(ctx context.Context, name string, args ...string) string {
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return "unknown"
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "unknown"
	}
	return name + "-" + value
}

func commandFailure(err error, output []byte) string {
	message := strings.TrimSpace(string(output))
	if len(message) > 1000 {
		message = message[len(message)-1000:]
	}
	if message == "" {
		return err.Error()
	}
	return message
}
