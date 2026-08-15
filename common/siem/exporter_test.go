package siem

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
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

func TestExporterUsesRealTLSRotatesTokenAndRetries(t *testing.T) {
	var firstAttempts atomic.Int32
	received := make(chan Event, 2)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		expectedToken := "siem-token-one"
		if event.Action == "second" {
			expectedToken = "siem-token-two"
		}
		if r.Header.Get("Authorization") != "Bearer "+expectedToken {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		if event.Action == "first" && firstAttempts.Add(1) == 1 {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		received <- event
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	temporary := t.TempDir()
	caPath := filepath.Join(temporary, "siem-ca.pem")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(caPath, caPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	tokenPath := filepath.Join(temporary, "siem-token")
	if err := os.WriteFile(tokenPath, []byte("siem-token-one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	exporter, err := New(Config{
		Enabled: true, Endpoint: server.URL, BearerTokenFile: tokenPath, CACertificate: caPath,
		Timeout: time.Second, QueueSize: 2, MaxAttempts: 3, RetryBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exporter.Start(ctx)
	if !exporter.Export(Event{Action: "first", Request: `{"password":"***"}`}) {
		t.Fatal("first event was not queued")
	}
	select {
	case event := <-received:
		if event.Action != "first" || firstAttempts.Load() != 2 {
			t.Fatalf("retry result = %#v, attempts=%d", event, firstAttempts.Load())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("retried TLS event was not delivered")
	}
	if err := os.WriteFile(tokenPath, []byte("siem-token-two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !exporter.Export(Event{Action: "second"}) {
		t.Fatal("second event was not queued")
	}
	select {
	case event := <-received:
		if event.Action != "second" {
			t.Fatalf("unexpected second event: %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("event using rotated token was not delivered")
	}
	if exporter.Dropped() != 0 || exporter.Failed() != 0 {
		t.Fatalf("unexpected counters: dropped=%d failed=%d", exporter.Dropped(), exporter.Failed())
	}
}
