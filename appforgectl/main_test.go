package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveConfigUsesOwnerOnlyPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	if err := saveConfig(path, config{BaseURL: "https://api.example.com", APIKey: "secret"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}
}

func TestClientRetriesMutationWithSameIdempotencyKey(t *testing.T) {
	requests := 0
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("authorization header is missing")
		}
		if request.Header.Get("Idempotency-Key") != "stable-key" {
			t.Fatal("idempotency key changed between retries")
		}
		if requests == 1 {
			return response(http.StatusServiceUnavailable, `{"code":"UNAVAILABLE","message":"retry"}`), nil
		}
		return response(http.StatusOK, `{"code":200,"msg":"OK","data":{"id":9}}`), nil
	})
	api := &client{baseURL: "https://api.example.com", apiKey: "test-key", http: &http.Client{Transport: transport}, retries: 1}
	var result struct {
		ID int64 `json:"id"`
	}
	if _, err := api.mutate(context.Background(), http.MethodPost, "/open/v1/builds", map[string]any{"appId": 1}, &result, "stable-key"); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || result.ID != 9 {
		t.Fatalf("requests=%d result=%+v", requests, result)
	}
}

func TestRunDoesNotPrintConfiguredAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	var stdout, stderr bytes.Buffer
	code := run([]string{"--config", path, "auth", "configure", "--base-url", "https://api.example.com", "--api-key", "super-secret"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run code=%d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "super-secret") || strings.Contains(stderr.String(), "super-secret") {
		t.Fatal("API key was printed")
	}
}

func TestLoadRuntimeConfigSupportsEnvironmentOnlyCI(t *testing.T) {
	t.Setenv("APPFORGE_BASE_URL", "https://ci.example.com")
	t.Setenv("APPFORGE_API_KEY", "ci-secret")

	cfg, err := loadRuntimeConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("loadRuntimeConfig() error = %v", err)
	}
	if cfg.BaseURL != "https://ci.example.com" || cfg.APIKey != "ci-secret" {
		t.Fatalf("loadRuntimeConfig() = %+v", cfg)
	}
}

func TestUploadFileSendsContentLength(t *testing.T) {
	payload := []byte("apk-payload")
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.ContentLength != int64(len(payload)) {
			t.Errorf("ContentLength = %d, want %d", request.ContentLength, len(payload))
		}
		body, _ := io.ReadAll(request.Body)
		if string(body) != string(payload) {
			t.Errorf("body = %q", body)
		}
		return response(http.StatusNoContent, ""), nil
	})}

	path := filepath.Join(t.TempDir(), "source.apk")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := uploadFile(context.Background(), client, "https://storage.example.com/source.apk", "application/vnd.android.package-archive", int64(len(payload)), file); err != nil {
		t.Fatal(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
