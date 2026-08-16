package egressproxy

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseAllowlist(t *testing.T) {
	allowlist, err := ParseAllowlist(strings.NewReader("# enterprise targets\napi.example.com:443 # SIEM\n*.storage.example.com:9443\n[2001:db8::1]:443\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct {
		host    string
		port    string
		allowed bool
	}{
		{"api.example.com", "443", true},
		{"bucket.storage.example.com", "9443", true},
		{"storage.example.com", "9443", false},
		{"bucket.storage.example.com", "443", false},
		{"2001:db8::1", "443", true},
	} {
		if actual := allowlist.Allows(target.host, target.port); actual != target.allowed {
			t.Fatalf("Allows(%q, %q)=%v, want %v", target.host, target.port, actual, target.allowed)
		}
	}
}

func TestParseAllowlistRejectsUnsafeRules(t *testing.T) {
	for _, value := range []string{"", "*:443", "https://example.com:443", "example.com", "*.127.0.0.1:443", "example.com:0"} {
		if _, err := ParseAllowlist(strings.NewReader(value)); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestHandlerAllowsApprovedTLSAndRejectsOtherTraffic(t *testing.T) {
	target := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("approved"))
	}))
	defer target.Close()
	targetURL, _ := url.Parse(target.URL)
	allowlist, err := ParseAllowlist(strings.NewReader(targetURL.Host))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(allowlist, 4, time.Second, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewServer(handler)
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)
	client := &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // test fixture certificate only
	}}
	response, err := client.Get(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("approved HTTPS status=%d", response.StatusCode)
	}

	plainResponse, err := client.Get("http://example.invalid/")
	if err != nil {
		t.Fatal(err)
	}
	defer plainResponse.Body.Close()
	if plainResponse.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("plaintext status=%d, want %d", plainResponse.StatusCode, http.StatusMethodNotAllowed)
	}

	blocked := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer blocked.Close()
	if _, err := client.Get(blocked.URL); err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Fatalf("expected unlisted CONNECT rejection, got %v", err)
	}
}
