package siem

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExporterDeliversRedactedEventWithRotatableTokenFile(t *testing.T) {
	received := make(chan Event, 1)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer local-siem-token" {
			t.Errorf("unexpected authorization header %q", r.Header.Get("Authorization"))
		}
		var event Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Error(err)
		}
		received <- event
		return &http.Response{StatusCode: http.StatusAccepted, Status: "202 Accepted", Body: io.NopCloser(&emptyReader{}), Header: make(http.Header)}, nil
	})
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("local-siem-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	exporter, err := New(Config{Enabled: true, Endpoint: "https://siem.example.test/events", BearerTokenFile: tokenPath, QueueSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	exporter.client.Transport = transport
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exporter.Start(ctx)
	want := Event{TenantID: 7, Path: "/admin/core/applications", Request: `{"password":"***"}`}
	if !exporter.Export(want) {
		t.Fatal("event was not queued")
	}
	select {
	case got := <-received:
		if got.TenantID != want.TenantID || got.Request != want.Request {
			t.Fatalf("unexpected event: %#v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SIEM event was not delivered")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type emptyReader struct{}

func (*emptyReader) Read([]byte) (int, error) { return 0, io.EOF }

func TestExporterRejectsPlainHTTPInProductionMode(t *testing.T) {
	if _, err := New(Config{Enabled: true, Endpoint: "http://siem.internal/events"}); err == nil {
		t.Fatal("expected insecure SIEM endpoint rejection")
	}
}
