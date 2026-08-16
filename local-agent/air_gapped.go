package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"appforge/common/airgap"
)

const fixedAirGappedExecutor = "/usr/local/bin/appforge-local-build"

type airGappedReplayMarker struct {
	PackageCode         string `json:"package_code"`
	NonceSHA256         string `json:"nonce_sha256"`
	ExportPackageSHA256 string `json:"export_package_sha256"`
	ResultPackageSHA256 string `json:"result_package_sha256"`
	ConsumedAt          int64  `json:"consumed_at"`
}

func airGappedBuildCommand(args []string) error {
	flags := flag.NewFlagSet("air-gapped-build", flag.ContinueOnError)
	taskPackage := flags.String("task-package", "", "absolute signed AIR_GAPPED task ZIP path")
	resultPackage := flags.String("result-package", "", "absolute new signed AIR_GAPPED result ZIP path")
	stateDir := flags.String("state-dir", defaultStateDir(), "absolute registered Agent state directory")
	secretRoot := flags.String("secret-root", "/etc/appforge/local-secrets", "absolute local-file Secret root")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("air-gapped-build does not accept positional arguments")
	}
	for label, value := range map[string]string{
		"task-package": *taskPackage, "result-package": *resultPackage, "state-dir": *stateDir, "secret-root": *secretRoot,
	} {
		if strings.TrimSpace(value) == "" || !filepath.IsAbs(value) {
			return fmt.Errorf("%s must be an absolute path", label)
		}
	}
	if filepath.Clean(*taskPackage) == filepath.Clean(*resultPackage) {
		return errors.New("task and result package paths must differ")
	}
	if err := requirePrivateRegularFile(*taskPackage); err != nil {
		return fmt.Errorf("validate task package: %w", err)
	}
	if err := requireNewPrivateOutput(*resultPackage); err != nil {
		return fmt.Errorf("validate result package: %w", err)
	}
	if err := requireFixedAirGappedExecutor(); err != nil {
		return err
	}
	current, err := loadState(*stateDir)
	if err != nil {
		return fmt.Errorf("load Agent state: %w", err)
	}
	if err := validateLocalState(*stateDir, &current); err != nil {
		return fmt.Errorf("validate Agent state: %w", err)
	}

	packageFile, size, packageSHA, envelope, err := inspectAirGappedTask(*taskPackage)
	if err != nil {
		return err
	}
	defer packageFile.Close()
	certificate, privateKey, err := verifyAirGappedTaskIdentity(&current, envelope)
	if err != nil {
		return err
	}
	replay, err := lockAirGappedReplay(*stateDir, &envelope.Manifest)
	if err != nil {
		return err
	}
	defer replay.close()

	workDir, err := os.MkdirTemp(*stateDir, ".air-gapped-work-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)
	if err := os.Chmod(workDir, 0o700); err != nil {
		return err
	}
	inputPaths, err := extractAirGappedInputs(packageFile, size, workDir)
	if err != nil {
		return err
	}
	bundle, err := decodeAirGappedBuildBundle(&envelope.Manifest, inputPaths)
	if err != nil {
		return err
	}
	secret, err := resolveLocalSigningSecret(*secretRoot, bundle.SigningSecretRef)
	if err != nil {
		return fmt.Errorf("resolve AIR_GAPPED signing Secret: %w", err)
	}
	defer secret.erase()

	result, runErr := runAirGappedExecutor(workDir, &envelope.Manifest, bundle, secret)
	resultEnvelope, outputPaths, err := createAirGappedResultEnvelope(
		&envelope.Manifest, certificate, privateKey, packageSHA, workDir, result, runErr,
	)
	if err != nil {
		return err
	}
	resultSHA, err := publishAirGappedResult(*resultPackage, resultEnvelope, outputPaths)
	if err != nil {
		return err
	}
	if err := replay.consume(airGappedReplayMarker{
		PackageCode: envelope.Manifest.PackageCode, NonceSHA256: airgap.Digest([]byte(envelope.Manifest.Nonce)),
		ExportPackageSHA256: packageSHA, ResultPackageSHA256: resultSHA, ConsumedAt: time.Now().UnixMilli(),
	}); err != nil {
		_ = os.Remove(*resultPackage)
		return fmt.Errorf("record AIR_GAPPED replay marker: %w", err)
	}
	fmt.Printf("AIR_GAPPED result written: %s sha256=%s status=%s\n", *resultPackage, resultSHA, resultEnvelope.Manifest.Status)
	return nil
}

func inspectAirGappedTask(path string) (*os.File, int64, string, *airgap.TaskEnvelope, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, "", nil, err
	}
	info, err := file.Stat()
	if err != nil || info.Size() <= 0 || info.Size() > airgap.MaxPackageBytes {
		file.Close()
		return nil, 0, "", nil, errors.New("AIR_GAPPED task package size is invalid")
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		file.Close()
		return nil, 0, "", nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return nil, 0, "", nil, err
	}
	envelope, err := airgap.ReadTaskPackage(file, info.Size(), nil)
	if err != nil {
		file.Close()
		return nil, 0, "", nil, fmt.Errorf("verify AIR_GAPPED task package: %w", err)
	}
	return file, info.Size(), hex.EncodeToString(hasher.Sum(nil)), envelope, nil
}

func verifyAirGappedTaskIdentity(current *state, envelope *airgap.TaskEnvelope) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	if current == nil || envelope == nil || current.AgentID != envelope.Manifest.AgentID {
		return nil, nil, errors.New("AIR_GAPPED package targets a different Agent")
	}
	now := time.Now()
	issuedAt, expiresAt := time.UnixMilli(envelope.Manifest.IssuedAt), time.UnixMilli(envelope.Manifest.ExpiresAt)
	if now.Before(issuedAt.Add(-5*time.Minute)) || !now.Before(expiresAt) {
		return nil, nil, errors.New("AIR_GAPPED package is not currently valid")
	}
	certificate, err := readStrictCertificate(current.Certificate)
	if err != nil {
		return nil, nil, fmt.Errorf("read Agent certificate: %w", err)
	}
	if strings.ToLower(certificate.SerialNumber.Text(16)) != strings.ToLower(envelope.Manifest.AgentCertificateSerial) {
		return nil, nil, errors.New("AIR_GAPPED package targets a different Agent certificate")
	}
	expectedIdentity := fmt.Sprintf("spiffe://appforge/tenant/%d/local-agent/%d", envelope.Manifest.TenantID, envelope.Manifest.AgentID)
	if len(certificate.URIs) != 1 || certificate.URIs[0].String() != expectedIdentity {
		return nil, nil, errors.New("Agent certificate identity differs from AIR_GAPPED package")
	}
	ca, err := readStrictCertificate(current.ClientCA)
	if err != nil {
		return nil, nil, fmt.Errorf("read Agent CA: %w", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	if _, err := certificate.Verify(x509.VerifyOptions{Roots: pool, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		return nil, nil, fmt.Errorf("verify Agent certificate: %w", err)
	}
	caPublicKey, ok := ca.PublicKey.(*ecdsa.PublicKey)
	if !ok || caPublicKey.Curve == nil || caPublicKey.Curve.Params().Name != "P-256" {
		return nil, nil, errors.New("Agent CA must use ECDSA P-256")
	}
	canonical, err := airgap.CanonicalJSON(envelope.Manifest)
	if err != nil {
		return nil, nil, err
	}
	if err := airgap.Verify(caPublicKey, canonical, envelope.Signature); err != nil {
		return nil, nil, fmt.Errorf("verify control-plane AIR_GAPPED signature: %w", err)
	}
	certificatePublicKey, ok := certificate.PublicKey.(*ecdsa.PublicKey)
	if !ok || certificatePublicKey.Curve == nil || certificatePublicKey.Curve.Params().Name != "P-256" {
		return nil, nil, errors.New("Agent certificate must use ECDSA P-256")
	}
	privateKey, err := loadECDSAKey(current.PrivateKey)
	if err != nil || privateKey.Curve == nil || privateKey.Curve.Params().Name != "P-256" ||
		privateKey.PublicKey.X.Cmp(certificatePublicKey.X) != 0 || privateKey.PublicKey.Y.Cmp(certificatePublicKey.Y) != 0 {
		return nil, nil, errors.New("Agent certificate and private key do not match")
	}
	return certificate, privateKey, nil
}

func extractAirGappedInputs(file *os.File, size int64, root string) (map[string]string, error) {
	result := map[string]string{}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	_, err := airgap.ReadTaskPackage(file, size, func(artifact airgap.Artifact, reader io.Reader) error {
		target := filepath.Join(root, filepath.FromSlash(artifact.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(output, reader)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil || written != artifact.SizeBytes {
			return errors.New("AIR_GAPPED input extraction was incomplete")
		}
		result[artifact.Path] = target
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("extract AIR_GAPPED task package: %w", err)
	}
	return result, nil
}

func decodeAirGappedBuildBundle(manifest *airgap.TaskManifest, paths map[string]string) (*buildManifest, error) {
	if manifest == nil {
		return nil, errors.New("AIR_GAPPED task manifest is required")
	}
	decoder := json.NewDecoder(strings.NewReader(string(manifest.Bundle)))
	decoder.DisallowUnknownFields()
	var bundle buildManifest
	if err := decoder.Decode(&bundle); err != nil {
		return nil, errors.New("AIR_GAPPED build bundle is not strict JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("AIR_GAPPED build bundle contains trailing data")
	}
	if bundle.SchemaVersion != protocolVersion || bundle.Task == nil || strings.TrimSpace(bundle.SigningSecretRef) == "" ||
		len(bundle.Inputs) != len(manifest.Inputs) || bundle.BlockedReason != "" || bundle.Task.ID != manifest.TaskID ||
		bundle.Task.TenantID != manifest.TenantID || bundle.Task.BuilderAttempt != manifest.BuilderAttempt {
		return nil, errors.New("AIR_GAPPED build bundle is incomplete")
	}
	byIdentity := make(map[string]airgap.Artifact, len(manifest.Inputs))
	for _, artifact := range manifest.Inputs {
		byIdentity[fmt.Sprintf("%s:%d", artifact.Role, artifact.ObjectID)] = artifact
	}
	for index := range bundle.Inputs {
		input := &bundle.Inputs[index]
		artifact, ok := byIdentity[fmt.Sprintf("%s:%d", input.Role, input.ObjectID)]
		if !ok || int32(input.ObjectType) != artifact.ObjectType || input.OriginalName != artifact.OriginalName ||
			input.ContentType != artifact.ContentType || input.SizeBytes != artifact.SizeBytes || input.SHA256 != artifact.SHA256 {
			return nil, errors.New("AIR_GAPPED signed bundle input differs from packaged Artifact")
		}
		input.LocalPath = paths[artifact.Path]
		input.DownloadPath, input.CustomerReference = "", ""
		input.StorageMode, input.OwnerAgentID = 0, 0
		delete(byIdentity, fmt.Sprintf("%s:%d", input.Role, input.ObjectID))
	}
	if len(byIdentity) != 0 {
		return nil, errors.New("AIR_GAPPED build bundle omits packaged inputs")
	}
	return &bundle, nil
}

func runAirGappedExecutor(workDir string, manifest *airgap.TaskManifest, bundle *buildManifest, secret *localSigningSecret) (*buildResult, error) {
	taskFile := filepath.Join(workDir, "task.json")
	resultFile := filepath.Join(workDir, "result.json")
	executorBundle := *bundle
	executorBundle.SigningSecretRef = ""
	executorBundle.Outputs = nil
	encoded, err := json.Marshal(map[string]any{"task": bundle.Task, "artifactMode": 3, "bundle": &executorBundle})
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(taskFile, encoded, 0o600); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.UnixMilli(manifest.ExpiresAt))
	defer cancel()
	command := exec.CommandContext(ctx, fixedAirGappedExecutor, "--task", taskFile, "--result", resultFile)
	command.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "HOME=" + workDir, "TMPDIR=" + workDir,
		"APPFORGE_TASK_ID=" + fmt.Sprint(manifest.TaskID), "APPFORGE_ARTIFACT_MODE=3",
		"APPFORGE_KEYSTORE_PASSWORD=" + secret.KeystorePassword, "APPFORGE_KEY_PASSWORD=" + secret.KeyPassword}
	output, runErr := command.CombinedOutput()
	result, decodeErr := readAirGappedExecutorResult(resultFile)
	if decodeErr != nil {
		if runErr == nil {
			runErr = decodeErr
		}
		result = &buildResult{}
	}
	if len(output) > 0 && runErr != nil {
		runErr = fmt.Errorf("%w; executor output sha256=%s", runErr, digestBytes(output))
	}
	return result, runErr
}

func readAirGappedExecutorResult(path string) (*buildResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 2<<20))
	decoder.DisallowUnknownFields()
	var result buildResult
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("local executor result contains trailing data")
	}
	return &result, nil
}

func createAirGappedResultEnvelope(manifest *airgap.TaskManifest, certificate *x509.Certificate, privateKey *ecdsa.PrivateKey,
	exportSHA, workDir string, result *buildResult, runErr error) (airgap.ResultEnvelope, map[string]string, error) {
	if result == nil {
		result = &buildResult{}
	}
	outputs := make([]airgap.Artifact, 0, 2)
	paths := map[string]string{}
	if runErr == nil && strings.TrimSpace(result.Error) == "" {
		artifact, err := executorOutputArtifact(workDir, "built_apk", "outputs/built.apk", result.APKPath,
			result.APKSize, result.APKSHA256, "built.apk", "application/vnd.android.package-archive")
		if err != nil {
			runErr = err
		} else {
			outputs = append(outputs, artifact)
			paths[artifact.Role] = result.APKPath
		}
	}
	if strings.TrimSpace(result.LogPath) != "" {
		artifact, err := executorOutputArtifact(workDir, "build_log", "outputs/build.log", result.LogPath,
			result.LogSize, result.LogSHA256, "build.log", "text/plain; charset=utf-8")
		if err == nil {
			outputs = append(outputs, artifact)
			paths[artifact.Role] = result.LogPath
		} else if runErr == nil {
			runErr = err
		}
	}
	statusValue, errorMessage := "SUCCESS", ""
	if runErr != nil || strings.TrimSpace(result.Error) != "" {
		statusValue = "FAILED"
		outputs = removeAirGappedAPK(outputs)
		delete(paths, "built_apk")
		errorMessage = sanitizeAirGappedError(result.Error)
		if errorMessage == "" && runErr != nil {
			errorMessage = sanitizeAirGappedError(runErr.Error())
		}
		if errorMessage == "" {
			errorMessage = "AIR_GAPPED_BUILD_FAILED"
		}
	}
	resultManifest := airgap.ResultManifest{SchemaVersion: airgap.SchemaVersion, PackageCode: manifest.PackageCode,
		Nonce: manifest.Nonce, TenantID: manifest.TenantID, AgentID: manifest.AgentID,
		AgentCertificateSerial: strings.ToLower(certificate.SerialNumber.Text(16)), TaskID: manifest.TaskID,
		BuilderAttempt: manifest.BuilderAttempt, ExportPackageSHA256: exportSHA, BuiltAt: time.Now().UnixMilli(),
		Status: statusValue, ErrorMessage: errorMessage, Outputs: outputs}
	canonical, err := airgap.CanonicalJSON(resultManifest)
	if err != nil {
		return airgap.ResultEnvelope{}, nil, err
	}
	signature, err := airgap.Sign(privateKey, canonical)
	if err != nil {
		return airgap.ResultEnvelope{}, nil, err
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
	envelope := airgap.ResultEnvelope{Manifest: resultManifest, CertificatePEM: string(certificatePEM), Signature: signature}
	if encoded, err := airgap.CanonicalJSON(envelope); err != nil {
		return airgap.ResultEnvelope{}, nil, err
	} else if _, err := airgap.DecodeResultEnvelope(encoded); err != nil {
		return airgap.ResultEnvelope{}, nil, err
	}
	return envelope, paths, nil
}

func executorOutputArtifact(root, role, packagePath, filename string, expectedSize int64, expectedSHA, originalName, contentType string) (airgap.Artifact, error) {
	resolved, err := requireWorkOutput(root, filename)
	if err != nil {
		return airgap.Artifact{}, err
	}
	file, err := os.Open(resolved)
	if err != nil {
		return airgap.Artifact{}, err
	}
	hasher := sha256.New()
	size, copyErr := io.Copy(hasher, io.LimitReader(file, airgap.MaxPackageBytes+1))
	closeErr := file.Close()
	digest := hex.EncodeToString(hasher.Sum(nil))
	if copyErr != nil || closeErr != nil || size <= 0 || size > airgap.MaxPackageBytes ||
		(expectedSize != 0 && expectedSize != size) || (expectedSHA != "" && expectedSHA != digest) {
		return airgap.Artifact{}, errors.New("local executor output integrity is invalid")
	}
	return airgap.Artifact{Role: role, Path: packagePath, OriginalName: originalName, ContentType: contentType,
		SizeBytes: size, SHA256: digest}, nil
}

func publishAirGappedResult(path string, envelope airgap.ResultEnvelope, outputs map[string]string) (string, error) {
	parent := filepath.Dir(path)
	temporary, err := os.CreateTemp(parent, ".air-gapped-result-*.tmp")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return "", err
	}
	err = airgap.WriteResultPackage(temporary, envelope, func(artifact airgap.Artifact) (io.ReadCloser, error) {
		return os.Open(outputs[artifact.Role])
	})
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return "", fmt.Errorf("publish AIR_GAPPED result without overwrite: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	_, copyErr := io.Copy(hasher, file)
	closeErr := file.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

type airGappedReplayLock struct {
	file       *os.File
	markerPath string
}

func lockAirGappedReplay(stateDir string, manifest *airgap.TaskManifest) (*airGappedReplayLock, error) {
	root := filepath.Join(stateDir, "air-gapped-replay")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, err
	}
	key := airgap.Digest([]byte(manifest.PackageCode + "\x00" + manifest.Nonce))
	lockFile, err := os.OpenFile(filepath.Join(root, key+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		lockFile.Close()
		return nil, err
	}
	markerPath := filepath.Join(root, key+".consumed.json")
	if _, err := os.Lstat(markerPath); err == nil {
		syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		lockFile.Close()
		return nil, errors.New("AIR_GAPPED task package was already consumed")
	} else if !os.IsNotExist(err) {
		syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		lockFile.Close()
		return nil, err
	}
	return &airGappedReplayLock{file: lockFile, markerPath: markerPath}, nil
}

func (lock *airGappedReplayLock) consume(marker airGappedReplayMarker) error {
	encoded, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	return writePrivateFile(lock.markerPath, encoded)
}

func (lock *airGappedReplayLock) close() {
	if lock == nil || lock.file == nil {
		return
	}
	_ = syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	_ = lock.file.Close()
}

func requireFixedAirGappedExecutor() error {
	info, err := os.Lstat(fixedAirGappedExecutor)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o111 == 0 || info.Mode().Perm()&0o022 != 0 {
		return errors.New("fixed AIR_GAPPED executor is unavailable or unsafe")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		return errors.New("fixed AIR_GAPPED executor must be owned by root")
	}
	return nil
}

func requirePrivateRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return errors.New("path must be a private regular file without symlinks")
	}
	return nil
}

func requireNewPrivateOutput(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return errors.New("output already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("output directory must be private")
	}
	return nil
}

func readStrictCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, trailing := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" || len(trailing) != 0 {
		return nil, errors.New("certificate PEM is invalid")
	}
	return x509.ParseCertificate(block.Bytes)
}

func requireWorkOutput(root, path string) (string, error) {
	if strings.TrimSpace(path) == "" || !filepath.IsAbs(path) {
		return "", errors.New("local executor output path must be absolute")
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || (resolved != root && !strings.HasPrefix(resolved, root+string(filepath.Separator))) {
		return "", errors.New("local executor output escapes the private work directory")
	}
	if err := requirePrivateRegularFile(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func sanitizeAirGappedError(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\x00", ""), "\r", ""))
	if len(value) > 2000 {
		value = value[:2000]
	}
	return value
}

func removeAirGappedAPK(items []airgap.Artifact) []airgap.Artifact {
	result := items[:0]
	for _, item := range items {
		if item.Role != "built_apk" {
			result = append(result, item)
		}
	}
	return result
}
