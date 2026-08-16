package worker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLocalExecutorTaskAndVerifyInputs(t *testing.T) {
	root := t.TempDir()
	source := writeLocalExecutorFixture(t, root, "source.apk", []byte("source-apk-fixture"))
	keystore := writeLocalExecutorFixture(t, root, "release.jks", []byte("keystore-fixture"))
	taskPath := filepath.Join(root, "task.json")
	resultPath := filepath.Join(root, "result.json")
	task := &localExecutorTask{ID: 31, TenantID: 7, AppID: 9, VersionID: 11, BuilderAttempt: 2,
		ChannelCode: "website", VersionCode: 100, VersionName: "1.0.0"}
	envelope := localExecutorEnvelope{Task: task, ArtifactMode: 1, Bundle: &localExecutorBundle{
		SchemaVersion: 3, Task: task, PackageName: "com.example.app", KeyAlias: "release",
		SignerCertificateSHA256: strings.Repeat("a", 64), Inputs: []localExecutorInput{
			localExecutorFixtureInput(t, "source_apk", 1, source), localExecutorFixtureInput(t, "keystore", 2, keystore),
		},
	}}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(taskPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	decoded, decodedRoot, err := readLocalExecutorTask(taskPath, resultPath)
	if err != nil {
		t.Fatal(err)
	}
	if decodedRoot != root || decoded.Task.ID != task.ID {
		t.Fatalf("unexpected decoded task: root=%q task=%#v", decodedRoot, decoded.Task)
	}
	for _, input := range decoded.Bundle.Inputs {
		if err := verifyLocalExecutorInput(root, input); err != nil {
			t.Fatalf("verify %s: %v", input.Role, err)
		}
	}
	if err := os.WriteFile(source, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyLocalExecutorInput(root, decoded.Bundle.Inputs[0]); err == nil {
		t.Fatal("tampered input was accepted")
	}
}

func TestReadLocalExecutorTaskAcceptsCustomerStorageMode(t *testing.T) {
	root := t.TempDir()
	taskPath := filepath.Join(root, "task.json")
	resultPath := filepath.Join(root, "result.json")
	task := &localExecutorTask{ID: 1, TenantID: 2, AppID: 3, BuilderAttempt: 4}
	envelope := localExecutorEnvelope{Task: task, ArtifactMode: 2, Bundle: &localExecutorBundle{
		SchemaVersion: localExecutorSchemaVersion, Task: task, PackageName: "com.example.app", KeyAlias: "release",
		SignerCertificateSHA256: strings.Repeat("a", 64),
	}}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(taskPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readLocalExecutorTask(taskPath, resultPath); err != nil {
		t.Fatalf("customer storage task rejected: %v", err)
	}
}

func TestReadLocalExecutorTaskAcceptsAirGappedMode(t *testing.T) {
	root := t.TempDir()
	taskPath := filepath.Join(root, "task.json")
	resultPath := filepath.Join(root, "result.json")
	task := &localExecutorTask{ID: 1, TenantID: 2, AppID: 3, BuilderAttempt: 4}
	envelope := localExecutorEnvelope{Task: task, ArtifactMode: 3, Bundle: &localExecutorBundle{
		SchemaVersion: localExecutorSchemaVersion, Task: task, PackageName: "com.example.app", KeyAlias: "release",
		SignerCertificateSHA256: strings.Repeat("a", 64),
	}}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(taskPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readLocalExecutorTask(taskPath, resultPath); err != nil {
		t.Fatalf("AIR_GAPPED task rejected: %v", err)
	}
}

func TestLocalExecutorRejectsPathsAndUnknownTaskFields(t *testing.T) {
	root := t.TempDir()
	outside := writeLocalExecutorFixture(t, t.TempDir(), "outside.apk", []byte("outside"))
	input := localExecutorFixtureInput(t, "source_apk", 1, outside)
	if err := verifyLocalExecutorInput(root, input); err == nil {
		t.Fatal("input outside task directory was accepted")
	}
	inside := writeLocalExecutorFixture(t, root, "inside.apk", []byte("inside"))
	linked := filepath.Join(root, "linked.apk")
	if err := os.Symlink(inside, linked); err != nil {
		t.Fatal(err)
	}
	input = localExecutorFixtureInput(t, "source_apk", 1, inside)
	input.LocalPath = linked
	if err := verifyLocalExecutorInput(root, input); err == nil {
		t.Fatal("symlinked input was accepted")
	}
	taskPath := filepath.Join(root, "task.json")
	if err := os.WriteFile(taskPath, []byte(`{"task":{},"artifactMode":1,"bundle":{},"command":"sh"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readLocalExecutorTask(taskPath, filepath.Join(root, "result.json")); err == nil {
		t.Fatal("unknown command field was accepted")
	}
}

func TestExecuteLocalTaskWritesFailureResultForInvalidBundle(t *testing.T) {
	root := t.TempDir()
	taskPath := filepath.Join(root, "task.json")
	resultPath := filepath.Join(root, "result.json")
	if err := os.WriteFile(taskPath, []byte(`{"task":null,"artifactMode":1,"bundle":null}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ExecuteLocalTask(context.Background(), taskPath, resultPath); err == nil {
		t.Fatal("invalid task unexpectedly succeeded")
	}
	raw, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	var result localExecutorResult
	if err := json.Unmarshal(raw, &result); err != nil || result.Error == "" {
		t.Fatalf("failure result is invalid: result=%#v err=%v", result, err)
	}
}

func TestExecuteLocalTaskRejectsUnsafeResultPathWithoutWriting(t *testing.T) {
	root := t.TempDir()
	taskPath := filepath.Join(root, "task.json")
	if err := os.WriteFile(taskPath, []byte(`{"task":null,"artifactMode":1,"bundle":null}`), 0o600); err != nil {
		t.Fatal(err)
	}
	outsideResult := root + ".tmp"
	if err := ExecuteLocalTask(context.Background(), taskPath, root); err == nil {
		t.Fatal("task directory was accepted as result path")
	}
	if _, err := os.Lstat(outsideResult); !os.IsNotExist(err) {
		t.Fatalf("unsafe result write escaped the task directory: %v", err)
	}
	symlinkResult := filepath.Join(root, "result.json")
	if err := os.Symlink(taskPath, symlinkResult); err != nil {
		t.Fatal(err)
	}
	if err := ExecuteLocalTask(context.Background(), taskPath, symlinkResult); err == nil {
		t.Fatal("symlink result path was accepted")
	}
}

func TestSecureLocalExecutorOutputMakesFilePrivateAndRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "channel.apk")
	if err := os.WriteFile(output, []byte("apk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := secureLocalExecutorOutput(output); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("output mode=%o", info.Mode().Perm())
	}
	linked := filepath.Join(root, "linked.apk")
	if err := os.Symlink(output, linked); err != nil {
		t.Fatal(err)
	}
	if err := secureLocalExecutorOutput(linked); err == nil {
		t.Fatal("symlinked executor output was accepted")
	}
}

func writeLocalExecutorFixture(t *testing.T, root, name string, value []byte) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, value, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func localExecutorFixtureInput(t *testing.T, role string, id int64, path string) localExecutorInput {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return localExecutorInput{Role: role, ObjectID: id, ObjectType: 1, OriginalName: filepath.Base(path),
		ContentType: "application/octet-stream", SizeBytes: info.Size(), SHA256: digestLocalExecutorBytes(raw), LocalPath: path}
}
