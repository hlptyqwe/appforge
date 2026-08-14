package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggedRequestURIRedactsWebhookTokenAndQuerySecrets(t *testing.T) {
	request := httptest.NewRequest("POST", "https://appforge.example.com/public/v1/source-webhooks/github/sensitive-token?code=oauth-code", nil)
	logged := loggedRequestURI(request)
	if logged != "/public/v1/source-webhooks/github/[REDACTED]" {
		t.Fatalf("loggedRequestURI() = %q", logged)
	}
	if strings.Contains(logged, "sensitive-token") || strings.Contains(logged, "oauth-code") {
		t.Fatalf("logged URI contains sensitive value: %q", logged)
	}

	callback := httptest.NewRequest("GET", "https://appforge.example.com/public/v1/source-oauth/callback?code=secret&state=signed", nil)
	if loggedRequestURI(callback) != "/public/v1/source-oauth/callback" {
		t.Fatalf("OAuth query was retained: %q", loggedRequestURI(callback))
	}
}
