package siem

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	if _, err := New(Config{Enabled: true, Endpoint: "syslog://siem.internal:514"}); err == nil {
		t.Fatal("expected plaintext syslog endpoint rejection")
	}
	for _, endpoint := range []string{
		"syslog+tls://user@siem.internal:6514",
		"syslog+tls://siem.internal:6514/events",
		"syslog+tls://siem.internal:6514?tenant=7",
		"syslog+tls://siem.internal",
	} {
		if _, err := New(Config{Enabled: true, Endpoint: endpoint}); err == nil {
			t.Fatalf("expected unsafe syslog TLS endpoint rejection: %s", endpoint)
		}
	}
}

func TestExporterSupportsControlledEnvironmentProxy(t *testing.T) {
	exporter, err := New(Config{Enabled: true, Endpoint: "https://siem.example.test/events"})
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := exporter.client.Transport.(*http.Transport)
	if !ok || transport.Proxy == nil {
		t.Fatal("SIEM exporter must support the controlled enterprise egress proxy")
	}
}

func TestSyslogTLSUsesCredentialFreeHTTPConnectProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://egress-proxy.internal:3128")
	t.Setenv("NO_PROXY", "")
	exporter, err := New(Config{Enabled: true, Endpoint: "syslog+tls://siem.customer.invalid:6514"})
	if err != nil {
		t.Fatal(err)
	}
	if exporter.proxy == nil || exporter.proxy.String() != "http://egress-proxy.internal:3128" {
		t.Fatalf("syslog TLS exporter did not select controlled CONNECT proxy: %v", exporter.proxy)
	}
	credentialProxy, err := url.Parse("http://user:password@egress-proxy.internal:3128")
	if err != nil {
		t.Fatal(err)
	}
	if err = validateSyslogProxy(credentialProxy); err == nil {
		t.Fatal("credential-bearing syslog CONNECT proxy was accepted")
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

type syslogTLSResult struct {
	message    string
	tlsVersion uint16
	err        error
}

func TestExporterUsesRealRFC5424SyslogTLSAndReconnects(t *testing.T) {
	seed := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	serverCertificate := seed.TLS.Certificates[0]
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: seed.Certificate().Raw})
	seed.Close()

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{serverCertificate},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var proxyConnections atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodConnect || request.Host != listener.Addr().String() {
			http.Error(writer, "invalid CONNECT target", http.StatusForbidden)
			return
		}
		upstream, dialErr := net.Dial("tcp", request.Host)
		if dialErr != nil {
			http.Error(writer, "target unavailable", http.StatusBadGateway)
			return
		}
		hijacker, ok := writer.(http.Hijacker)
		if !ok {
			_ = upstream.Close()
			http.Error(writer, "hijacking unavailable", http.StatusInternalServerError)
			return
		}
		client, _, hijackErr := hijacker.Hijack()
		if hijackErr != nil {
			_ = upstream.Close()
			return
		}
		proxyConnections.Add(1)
		_, _ = io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n")
		go func() {
			_, _ = io.Copy(upstream, client)
			_ = upstream.Close()
		}()
		_, _ = io.Copy(client, upstream)
		_ = client.Close()
	}))
	defer proxy.Close()
	results := make(chan syslogTLSResult, 2)
	go func() {
		for index := 0; index < 2; index++ {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				results <- syslogTLSResult{err: acceptErr}
				continue
			}
			tlsConnection, ok := connection.(*tls.Conn)
			if !ok {
				_ = connection.Close()
				results <- syslogTLSResult{err: net.InvalidAddrError("accepted connection is not TLS")}
				continue
			}
			if handshakeErr := tlsConnection.Handshake(); handshakeErr != nil {
				_ = connection.Close()
				results <- syslogTLSResult{err: handshakeErr}
				continue
			}
			reader := bufio.NewReader(tlsConnection)
			lengthText, readErr := reader.ReadString(' ')
			if readErr != nil {
				_ = connection.Close()
				results <- syslogTLSResult{err: readErr}
				continue
			}
			length, parseErr := strconv.Atoi(strings.TrimSuffix(lengthText, " "))
			if parseErr != nil || length <= 0 {
				_ = connection.Close()
				results <- syslogTLSResult{err: parseErr}
				continue
			}
			message := make([]byte, length)
			_, readErr = io.ReadFull(reader, message)
			state := tlsConnection.ConnectionState()
			_ = connection.Close()
			results <- syslogTLSResult{message: string(message), tlsVersion: state.Version, err: readErr}
		}
	}()

	temporary := t.TempDir()
	caPath := filepath.Join(temporary, "syslog-ca.pem")
	if err = os.WriteFile(caPath, caPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	exporter, err := New(Config{
		Enabled: true, Endpoint: "syslog+tls://" + listener.Addr().String(), CACertificate: caPath,
		Timeout: time.Second, QueueSize: 2, MaxAttempts: 2, RetryBackoff: time.Millisecond,
		SyslogHostname: "api-1.customer.test", SyslogAppName: "appforge-admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	exporter.proxy = proxyURL
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exporter.Start(ctx)
	events := []Event{
		{Timestamp: 1786896000123, TenantID: 7, UserID: 9, Module: "system", Action: "role]grant", Request: `{"password":"***"}`},
		{Timestamp: 1786896001123, TenantID: 7, UserID: 9, Module: "system", Action: "second", Response: `{"token":"***"}`},
	}
	for _, event := range events {
		if !exporter.Export(event) {
			t.Fatal("syslog TLS event was not queued")
		}
	}
	for index := range events {
		select {
		case result := <-results:
			if result.err != nil {
				t.Fatal(result.err)
			}
			if result.tlsVersion < tls.VersionTLS12 {
				t.Fatalf("insecure TLS version: %x", result.tlsVersion)
			}
			if !strings.HasPrefix(result.message, "<134>1 2026-08-16T") ||
				!strings.Contains(result.message, " api-1.customer.test appforge-admin - AUDIT ") ||
				!strings.Contains(result.message, `[appforge@32473 tenantId="7" userId="9" module="system"`) {
				t.Fatalf("invalid RFC5424 message: %q", result.message)
			}
			if !strings.Contains(result.message, `"tenantId":7`) || !strings.Contains(result.message, `"userId":9`) {
				t.Fatalf("missing JSON audit payload: %q", result.message)
			}
			if index == 0 && !strings.Contains(result.message, `action="role\]grant"`) {
				t.Fatalf("structured data was not escaped: %q", result.message)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("SIEM RFC5424 TLS event was not delivered")
		}
	}
	if exporter.Dropped() != 0 || exporter.Failed() != 0 {
		t.Fatalf("unexpected counters: dropped=%d failed=%d", exporter.Dropped(), exporter.Failed())
	}
	if proxyConnections.Load() != 2 {
		t.Fatalf("syslog TLS did not use a fresh CONNECT tunnel per event: %d", proxyConnections.Load())
	}
}

func TestExporterRejectsOversizedRFC5424Message(t *testing.T) {
	exporter, err := New(Config{
		Enabled: true, Endpoint: "syslog+tls://127.0.0.1:6514", MaxMessageBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = exporter.rfc5424Message(Event{Request: strings.Repeat("x", 2048)}); err == nil {
		t.Fatal("oversized RFC5424 message was accepted")
	}
}
