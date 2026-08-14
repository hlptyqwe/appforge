package logic

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestValidateWebhookEndpointURLRejectsPrivateTargets(t *testing.T) {
	invalid := []string{
		"http://example.com/hook",
		"https://127.0.0.1/hook",
		"https://10.0.0.8/hook",
		"https://169.254.169.254/latest/meta-data",
		"https://metadata.google.internal/hook",
	}
	for _, value := range invalid {
		if err := validateWebhookEndpointURL(context.Background(), value); err == nil {
			t.Fatalf("validateWebhookEndpointURL(%q) unexpectedly succeeded", value)
		}
	}
}

func TestWebhookSignature(t *testing.T) {
	body := []byte(`{"eventId":"event-1"}`)
	actual := webhookSignature("secret", "1700000000", body)
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write([]byte("1700000000."))
	_, _ = mac.Write(body)
	expected := "v1=" + hex.EncodeToString(mac.Sum(nil))
	if actual != expected {
		t.Fatalf("signature = %q, want %q", actual, expected)
	}
}

func TestSendWebhookCanRecoverAfterFailure(t *testing.T) {
	var attempts atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		timestamp := r.Header.Get("X-AppForge-Timestamp")
		if r.Header.Get("X-AppForge-Signature") != webhookSignature("test-secret", timestamp, body) {
			t.Error("invalid webhook signature")
		}
		if attempts.Add(1) == 1 {
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("temporary failure"))}, nil
		}
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}

	statusCode, excerpt, err := sendWebhookWithClient(context.Background(), client, "https://example.com/webhook", "event-1", "test-secret", []byte(`{"ok":true}`))
	if err == nil || statusCode != http.StatusServiceUnavailable || !strings.Contains(excerpt, "temporary") {
		t.Fatalf("first delivery = (%d, %q, %v)", statusCode, excerpt, err)
	}
	statusCode, _, err = sendWebhookWithClient(context.Background(), client, "https://example.com/webhook", "event-1", "test-secret", []byte(`{"ok":true}`))
	if err != nil || statusCode != http.StatusNoContent {
		t.Fatalf("second delivery = (%d, %v)", statusCode, err)
	}
	if webhookRetryDelay(1) != 2 || webhookRetryDelay(20) != 1024 {
		t.Fatal("unexpected exponential retry delay")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
