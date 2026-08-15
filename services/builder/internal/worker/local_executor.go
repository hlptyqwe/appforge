package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"appforge/proto/core"
)

const localExecutorSchemaVersion int32 = 3

type localExecutorEnvelope struct {
	Task         *localExecutorTask   `json:"task"`
	ArtifactMode int32                `json:"artifactMode"`
	Bundle       *localExecutorBundle `json:"bundle"`
}

type localExecutorTask struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenant_id"`
	AppID          int64  `json:"app_id"`
	VersionID      int64  `json:"version_id"`
	BuilderAttempt int32  `json:"builder_attempt"`
	ChannelCode    string `json:"channel_code"`
	VersionCode    int64  `json:"version_code"`
	VersionName    string `json:"version_name"`
}

type localExecutorBundle struct {
	SchemaVersion           int32                `json:"schema_version"`
	Task                    *localExecutorTask   `json:"task"`
	PackageName             string               `json:"package_name"`
	APIHost                 string               `json:"api_host"`
	ChannelName             string               `json:"channel_name"`
	LandingURL              string               `json:"landing_url"`
	KeyAlias                string               `json:"key_alias"`
	SignerCertificateSHA256 string               `json:"signer_certificate_sha256"`
	BrandingSnapshotJSON    string               `json:"branding_snapshot_json"`
	TemplateSnapshotJSON    string               `json:"template_snapshot_json"`
	Inputs                  []localExecutorInput `json:"inputs"`
	BlockedReason           string               `json:"blocked_reason"`
}

type localExecutorInput struct {
	Role         string `json:"role"`
	ObjectID     int64  `json:"object_id"`
	ObjectType   int32  `json:"object_type"`
	OriginalName string `json:"original_name"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
	LocalPath    string `json:"local_path"`
}

type localExecutorResult struct {
	APKPath   string `json:"apkPath,omitempty"`
	APKSHA256 string `json:"apkSha256,omitempty"`
	APKSize   int64  `json:"apkSize,omitempty"`
	LogPath   string `json:"logPath,omitempty"`
	LogSHA256 string `json:"logSha256,omitempty"`
	LogSize   int64  `json:"logSize,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ExecuteLocalTask runs the fixed local APK pipeline. The task contract only
// contains data; executable names and arguments are owned by this binary.
func ExecuteLocalTask(ctx context.Context, taskFile, resultFile string) error {
	if absoluteResult, err := filepath.Abs(resultFile); err == nil {
		resultFile = absoluteResult
	}
	envelope, root, err := readLocalExecutorTask(taskFile, resultFile)
	if err != nil {
		if root != "" {
			_ = writeLocalExecutorResult(resultFile, &localExecutorResult{Error: localExecutorError(err)})
		}
		return err
	}
	logPath := filepath.Join(root, "build.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		_ = writeLocalExecutorResult(resultFile, &localExecutorResult{Error: localExecutorError(err)})
		return err
	}
	result, executeErr := executeLocalPipeline(ctx, root, logFile, envelope)
	closeErr := logFile.Close()
	if executeErr == nil {
		executeErr = closeErr
	}
	if logSize, logSHA, digestErr := fileDigest(logPath); digestErr == nil {
		result.LogPath, result.LogSize, result.LogSHA256 = logPath, logSize, logSHA
	}
	if executeErr != nil {
		result.Error = localExecutorError(executeErr)
	}
	if err := writeLocalExecutorResult(resultFile, result); err != nil {
		return err
	}
	return executeErr
}

func readLocalExecutorTask(taskFile, resultFile string) (*localExecutorEnvelope, string, error) {
	if strings.TrimSpace(taskFile) == "" || strings.TrimSpace(resultFile) == "" {
		return nil, "", errors.New("task and result paths are required")
	}
	taskPath, err := filepath.Abs(taskFile)
	if err != nil {
		return nil, "", err
	}
	root := filepath.Dir(taskPath)
	if err := requireLocalExecutorPath(root, taskPath, true); err != nil {
		return nil, "", fmt.Errorf("validate task file: %w", err)
	}
	resultPath, err := filepath.Abs(resultFile)
	if err != nil || filepath.Dir(resultPath) != root || resultPath == taskPath {
		return nil, "", errors.New("result path must be a direct file inside the task directory")
	}
	if info, statErr := os.Lstat(resultPath); statErr == nil &&
		(info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0) {
		return nil, "", errors.New("existing result must be a private regular file")
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return nil, "", errors.New("inspect result path failed")
	}
	file, err := os.Open(taskPath)
	if err != nil {
		return nil, root, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 2<<20))
	decoder.DisallowUnknownFields()
	var envelope localExecutorEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return nil, root, fmt.Errorf("decode task bundle: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, root, errors.New("task bundle contains trailing data")
	}
	if envelope.Task == nil || envelope.Bundle == nil || envelope.Bundle.Task == nil ||
		envelope.Bundle.SchemaVersion != localExecutorSchemaVersion || envelope.ArtifactMode != 1 ||
		envelope.Task.ID <= 0 || envelope.Task.TenantID <= 0 || envelope.Task.AppID <= 0 || envelope.Task.BuilderAttempt <= 0 ||
		envelope.Bundle.Task.ID != envelope.Task.ID || envelope.Bundle.Task.TenantID != envelope.Task.TenantID ||
		envelope.Bundle.Task.AppID != envelope.Task.AppID || envelope.Bundle.Task.BuilderAttempt != envelope.Task.BuilderAttempt {
		return nil, root, errors.New("task bundle identity, schema or Artifact mode is invalid")
	}
	if envelope.Bundle.BlockedReason != "" || strings.TrimSpace(envelope.Bundle.PackageName) == "" ||
		strings.TrimSpace(envelope.Bundle.KeyAlias) == "" || len(strings.TrimSpace(envelope.Bundle.SignerCertificateSHA256)) != 64 {
		return nil, root, errors.New("task bundle is blocked or incomplete")
	}
	return &envelope, root, nil
}

func executeLocalPipeline(ctx context.Context, root string, logFile io.Writer, envelope *localExecutorEnvelope) (*localExecutorResult, error) {
	inputs := make(map[string][]localExecutorInput)
	for _, input := range envelope.Bundle.Inputs {
		if err := verifyLocalExecutorInput(root, input); err != nil {
			return &localExecutorResult{}, fmt.Errorf("verify %s input: %w", input.Role, err)
		}
		inputs[input.Role] = append(inputs[input.Role], input)
	}
	source, err := oneLocalExecutorInput(inputs, "source_apk")
	if err != nil {
		return &localExecutorResult{}, err
	}
	keystore, err := oneLocalExecutorInput(inputs, "keystore")
	if err != nil {
		return &localExecutorResult{}, err
	}
	branding, err := decodeBuildBrandingSnapshot(envelope.Bundle.BrandingSnapshotJSON)
	if err != nil {
		return &localExecutorResult{}, err
	}
	whiteLabel, err := decodeWhiteLabelBuildSnapshot(envelope.Bundle.TemplateSnapshotJSON)
	if err != nil {
		return &localExecutorResult{}, err
	}
	if whiteLabel != nil {
		if err := whiteLabel.decryptSensitiveParameters(func(string) (string, error) {
			return "", errors.New("control-plane encrypted template parameters are forbidden in Local Agent tasks")
		}); err != nil {
			return &localExecutorResult{}, err
		}
	}
	templateFiles := make(map[int64]string)
	for _, input := range inputs["template_file"] {
		templateFiles[input.ObjectID] = input.LocalPath
	}
	intermediate := source.LocalPath
	if branding != nil {
		logo, logoErr := oneLocalExecutorInput(inputs, "brand_logo")
		if logoErr != nil {
			return &localExecutorResult{}, logoErr
		}
		splash, splashErr := oneLocalExecutorInput(inputs, "brand_splash")
		if splashErr != nil {
			return &localExecutorResult{}, splashErr
		}
		intermediate, err = buildBrandedAPK(ctx, logFile, source.LocalPath, logo.LocalPath, splash.LocalPath, root,
			branding, whiteLabel, templateFiles, localExecutorStorageObject(logo), localExecutorStorageObject(splash))
		if err != nil {
			return &localExecutorResult{}, err
		}
	}
	unsignedPath := filepath.Join(root, "unsigned.apk")
	task := envelope.Task
	if err := injectChannelAsset(intermediate, unsignedPath, channelPayload{
		SchemaVersion: 1, TenantID: task.TenantID, AppID: task.AppID, VersionID: task.VersionID,
		BuildTaskID: task.ID, ChannelCode: task.ChannelCode, ChannelName: envelope.Bundle.ChannelName,
		APIHost: envelope.Bundle.APIHost, LandingURL: envelope.Bundle.LandingURL,
		VersionCode: task.VersionCode, VersionName: task.VersionName, BuildTime: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return &localExecutorResult{}, err
	}
	alignedPath := filepath.Join(root, "aligned.apk")
	signedPath := filepath.Join(root, "channel.apk")
	if err := runAndLog(ctx, logFile, nil, "zipalign", "-f", "4", unsignedPath, alignedPath); err != nil {
		return &localExecutorResult{}, fmt.Errorf("zipalign APK: %w", err)
	}
	storePassword := os.Getenv("APPFORGE_KEYSTORE_PASSWORD")
	keyPassword := os.Getenv("APPFORGE_KEY_PASSWORD")
	if storePassword == "" || keyPassword == "" {
		return &localExecutorResult{}, errors.New("local signing password environment is incomplete")
	}
	if err := runAndLog(ctx, logFile, nil, "keytool", "-list", "-keystore", keystore.LocalPath,
		"-storepass:env", "APPFORGE_KEYSTORE_PASSWORD", "-alias", envelope.Bundle.KeyAlias); err != nil {
		return &localExecutorResult{}, fmt.Errorf("validate Keystore alias or password: %w", err)
	}
	if err := verifyKeystoreCertificate(ctx, keystore.LocalPath, envelope.Bundle.KeyAlias, storePassword,
		envelope.Bundle.SignerCertificateSHA256); err != nil {
		return &localExecutorResult{}, err
	}
	if err := runAndLog(ctx, logFile, nil, "apksigner", "sign", "--ks", keystore.LocalPath,
		"--ks-key-alias", envelope.Bundle.KeyAlias, "--ks-pass", "env:APPFORGE_KEYSTORE_PASSWORD",
		"--key-pass", "env:APPFORGE_KEY_PASSWORD", "--out", signedPath, alignedPath); err != nil {
		return &localExecutorResult{}, fmt.Errorf("sign APK: %w", err)
	}
	if err := runAndLog(ctx, logFile, nil, "apksigner", "verify", "--verbose", "--print-certs", signedPath); err != nil {
		return &localExecutorResult{}, fmt.Errorf("verify APK signature: %w", err)
	}
	if err := verifyPackageName(ctx, logFile, signedPath, envelope.Bundle.PackageName); err != nil {
		return &localExecutorResult{}, err
	}
	if err := verifyAPKAsset(signedPath, channelAssetPath, `"channelCode":"`+task.ChannelCode+`"`); err != nil {
		return &localExecutorResult{}, err
	}
	if branding != nil {
		if err := verifyAPKAsset(signedPath, brandingAssetPath, `"revision":`+fmt.Sprint(branding.Revision)); err != nil {
			return &localExecutorResult{}, err
		}
	}
	if whiteLabel != nil {
		if err := verifyAPKAsset(signedPath, whiteLabelAssetPath, `"templateChecksum":"`+whiteLabel.TemplateChecksum+`"`); err != nil {
			return &localExecutorResult{}, err
		}
	}
	size, digest, err := fileDigest(signedPath)
	if err != nil {
		return &localExecutorResult{}, err
	}
	return &localExecutorResult{APKPath: signedPath, APKSize: size, APKSHA256: digest}, nil
}

func verifyLocalExecutorInput(root string, input localExecutorInput) error {
	if input.ObjectID <= 0 || input.SizeBytes <= 0 || strings.TrimSpace(input.Role) == "" || len(strings.TrimSpace(input.SHA256)) != 64 {
		return errors.New("input metadata is incomplete")
	}
	path, err := filepath.Abs(input.LocalPath)
	if err != nil {
		return err
	}
	if err := requireLocalExecutorPath(root, path, true); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() != input.SizeBytes {
		return errors.New("input file size does not match manifest")
	}
	size, digest, err := fileDigest(path)
	if err != nil || size != input.SizeBytes || digest != strings.ToLower(strings.TrimSpace(input.SHA256)) {
		return errors.New("input file SHA-256 does not match manifest")
	}
	return nil
}

func requireLocalExecutorPath(root, path string, mustExist bool) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return errors.New("resolve task directory failed")
	}
	path, err = filepath.Abs(path)
	if err != nil || (path != root && !strings.HasPrefix(path, root+string(filepath.Separator))) {
		return errors.New("path escapes task directory")
	}
	if !mustExist {
		return nil
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("task file must be a private regular file")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || (resolved != resolvedRoot && !strings.HasPrefix(resolved, resolvedRoot+string(filepath.Separator))) {
		return errors.New("resolved task path escapes task directory")
	}
	return nil
}

func oneLocalExecutorInput(inputs map[string][]localExecutorInput, role string) (localExecutorInput, error) {
	items := inputs[role]
	if len(items) != 1 {
		return localExecutorInput{}, fmt.Errorf("exactly one %s input is required", role)
	}
	return items[0], nil
}

func localExecutorStorageObject(input localExecutorInput) *core.StorageObject {
	return &core.StorageObject{Id: input.ObjectID, ObjectType: core.StorageObjectType(input.ObjectType),
		OriginalName: input.OriginalName, ContentType: input.ContentType, SizeBytes: input.SizeBytes, Sha256: input.SHA256}
}

func writeLocalExecutorResult(path string, result *localExecutorResult) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func localExecutorError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 2000 {
		message = message[:2000]
	}
	return message
}

func digestLocalExecutorBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
