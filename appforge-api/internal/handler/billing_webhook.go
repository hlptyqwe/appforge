package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"appforge/admin-api/internal/svc"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/rest"
)

const stripeWebhookBodyLimit = 2 << 20

func RegisterBillingHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoute(rest.Route{Method: http.MethodPost, Path: "/public/v1/billing/stripe", Handler: stripeWebhookHandler(serverCtx)})
}

func stripeWebhookHandler(serverCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := strings.TrimSpace(serverCtx.Config.Billing.StripeWebhookSecret)
		if secret == "" {
			http.Error(w, "billing webhook is not configured", http.StatusServiceUnavailable)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, stripeWebhookBodyLimit))
		if err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if err := verifyStripeSignature(body, r.Header.Get("Stripe-Signature"), secret, time.Now()); err != nil {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
		normalized, err := normalizeStripeEvent(body)
		if err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		if _, err := serverCtx.CoreCli.ApplyBillingWebhook(r.Context(), normalized); err != nil {
			http.Error(w, "event processing failed", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"received":true}`))
	}
}

func verifyStripeSignature(payload []byte, header, secret string, now time.Time) error {
	var timestamp int64
	var signatures []string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			timestamp, _ = strconv.ParseInt(value, 10, 64)
		case "v1":
			signatures = append(signatures, value)
		}
	}
	if timestamp <= 0 || len(signatures) == 0 || now.Sub(time.Unix(timestamp, 0)) > 5*time.Minute || time.Unix(timestamp, 0).Sub(now) > 5*time.Minute {
		return fmt.Errorf("invalid Stripe signature timestamp")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d.", timestamp)
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)
	for _, signature := range signatures {
		decoded, err := hex.DecodeString(signature)
		if err == nil && subtle.ConstantTimeCompare(decoded, expected) == 1 {
			return nil
		}
	}
	return fmt.Errorf("Stripe signature mismatch")
}

func normalizeStripeEvent(payload []byte) (*core.ApplyBillingWebhookReq, error) {
	var event struct {
		Id      string `json:"id"`
		Type    string `json:"type"`
		Created int64  `json:"created"`
		Data    struct {
			Object map[string]any `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &event); err != nil || event.Id == "" || event.Type == "" || event.Created <= 0 || event.Data.Object == nil {
		return nil, fmt.Errorf("invalid Stripe event envelope")
	}
	object := event.Data.Object
	metadata := mapAt(object, "metadata")
	if len(metadata) == 0 {
		metadata = mapAt(object, "subscription_details", "metadata")
	}
	if len(metadata) == 0 {
		metadata = mapAt(object, "parent", "subscription_details", "metadata")
	}
	tenantID, _ := strconv.ParseInt(stringValueAt(metadata, "tenant_id"), 10, 64)
	planID, _ := strconv.ParseInt(stringValueAt(metadata, "plan_id"), 10, 64)
	externalSubscriptionID := stringValueAt(object, "subscription")
	if strings.HasPrefix(event.Type, "customer.subscription.") {
		externalSubscriptionID = stringValueAt(object, "id")
	}
	if externalSubscriptionID == "" {
		externalSubscriptionID = stringValueAt(object, "parent", "subscription_details", "subscription")
	}
	externalInvoiceID := stringValueAt(object, "invoice")
	if strings.HasPrefix(event.Type, "invoice.") {
		externalInvoiceID = stringValueAt(object, "id")
	}
	transactionID := firstStringAt(object, []string{"charge"}, []string{"payment_intent"}, []string{"id"})
	amount := firstIntAt(object, []string{"amount_paid"}, []string{"amount_total"}, []string{"amount"})
	if event.Type == "charge.refunded" {
		refund := latestStripeRefund(object)
		if refund != nil {
			transactionID = stringValueAt(refund, "id")
			amount = intValueAt(refund, "amount")
		}
	}
	if event.Type == "charge.dispute.created" {
		externalInvoiceID = stringValueAt(object, "charge")
		transactionID = stringValueAt(object, "id")
	}
	periodStart := firstIntAt(object, []string{"current_period_start"}, []string{"period_start"}, []string{"lines", "data", "0", "period", "start"})
	periodEnd := firstIntAt(object, []string{"current_period_end"}, []string{"period_end"}, []string{"lines", "data", "0", "period", "end"})
	return &core.ApplyBillingWebhookReq{
		Provider: "stripe", ProviderEventId: event.Id, EventType: event.Type, EventCreatedAt: event.Created,
		PayloadJson: string(payload), TenantId: tenantID, PlanId: planID,
		ExternalCustomerId: stringValueAt(object, "customer"), ExternalSubscriptionId: externalSubscriptionID,
		ExternalInvoiceId: externalInvoiceID, ExternalTransactionId: transactionID,
		Currency: strings.ToUpper(stringValueAt(object, "currency")), Amount: amount,
		PeriodStart: periodStart, PeriodEnd: periodEnd, CancelAtPeriodEnd: boolValueAt(object, "cancel_at_period_end"),
		FailureCode:    firstStringAt(object, []string{"failure_code"}, []string{"last_payment_error", "code"}),
		FailureMessage: firstStringAt(object, []string{"failure_message"}, []string{"last_payment_error", "message"}),
	}, nil
}

func valueAt(value any, path ...string) any {
	current := value
	for _, part := range path {
		switch typed := current.(type) {
		case map[string]any:
			current = typed[part]
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(typed) {
				return nil
			}
			current = typed[index]
		default:
			return nil
		}
	}
	return current
}

func mapAt(value any, path ...string) map[string]any {
	result, _ := valueAt(value, path...).(map[string]any)
	return result
}

func stringValueAt(value any, path ...string) string {
	result := valueAt(value, path...)
	if text, ok := result.(string); ok {
		return text
	}
	if object, ok := result.(map[string]any); ok {
		if id, ok := object["id"].(string); ok {
			return id
		}
	}
	return ""
}

func firstStringAt(value any, paths ...[]string) string {
	for _, path := range paths {
		if result := stringValueAt(value, path...); result != "" {
			return result
		}
	}
	return ""
}

func intValueAt(value any, path ...string) int64 {
	switch result := valueAt(value, path...).(type) {
	case float64:
		return int64(result)
	case json.Number:
		value, _ := result.Int64()
		return value
	}
	return 0
}

func firstIntAt(value any, paths ...[]string) int64 {
	for _, path := range paths {
		if result := intValueAt(value, path...); result != 0 {
			return result
		}
	}
	return 0
}

func boolValueAt(value any, path ...string) bool {
	result, _ := valueAt(value, path...).(bool)
	return result
}

func latestStripeRefund(object map[string]any) map[string]any {
	items, _ := valueAt(object, "refunds", "data").([]any)
	var latest map[string]any
	for _, item := range items {
		candidate, _ := item.(map[string]any)
		if candidate != nil && (latest == nil || intValueAt(candidate, "created") > intValueAt(latest, "created")) {
			latest = candidate
		}
	}
	return latest
}
