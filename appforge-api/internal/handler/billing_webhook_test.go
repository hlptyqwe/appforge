package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func TestVerifyStripeSignature(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	payload := []byte(`{"id":"evt_1"}`)
	mac := hmac.New(sha256.New, []byte("whsec_test"))
	_, _ = fmt.Fprintf(mac, "%d.", now.Unix())
	_, _ = mac.Write(payload)
	header := fmt.Sprintf("t=%d,v1=%s", now.Unix(), hex.EncodeToString(mac.Sum(nil)))
	if err := verifyStripeSignature(payload, header, "whsec_test", now); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := verifyStripeSignature(payload, header, "wrong", now); err == nil {
		t.Fatal("wrong secret was accepted")
	}
	if err := verifyStripeSignature(payload, header, "whsec_test", now.Add(6*time.Minute)); err == nil {
		t.Fatal("stale signature was accepted")
	}
}

func TestNormalizeStripeSubscriptionEvent(t *testing.T) {
	payload := []byte(`{
  "id":"evt_subscription","type":"customer.subscription.updated","created":1700000000,
  "data":{"object":{"id":"sub_1","customer":"cus_1","currency":"usd",
    "current_period_start":1700000000,"current_period_end":1702678400,"cancel_at_period_end":true,
    "metadata":{"tenant_id":"42","plan_id":"7"}}}}
`)
	event, err := normalizeStripeEvent(payload)
	if err != nil {
		t.Fatal(err)
	}
	if event.TenantId != 42 || event.PlanId != 7 || event.ExternalSubscriptionId != "sub_1" ||
		event.ExternalCustomerId != "cus_1" || event.Currency != "USD" || !event.CancelAtPeriodEnd {
		t.Fatalf("unexpected normalized event: %+v", event)
	}
}
