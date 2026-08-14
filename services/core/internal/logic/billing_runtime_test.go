package logic

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"appforge/common/secretbox"
	"appforge/common/utils"
	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBillingRuntimeAcceptance(t *testing.T) {
	dsn := os.Getenv("APPFORGE_BILLING_TEST_DSN")
	if dsn == "" {
		t.Skip("APPFORGE_BILLING_TEST_DSN is not set")
	}
	db := sqlx.NewMysql(dsn)
	box, err := secretbox.New("YXBwZm9yZ2UtZGV2LXNpZ25pbmcta2V5LTAwMDAwMSE=")
	if err != nil {
		t.Fatal(err)
	}
	svcCtx := &svc.ServiceContext{DB: db, Secrets: box}
	tenant := time.Now().UnixNano()/1000%900_000_000 + 8_000_000_000
	ctx := context.WithValue(context.Background(), utils.CtxKeyTenantId, tenant)
	prefix := fmt.Sprintf("v6-runtime-%d", tenant)
	t.Cleanup(func() { cleanupBillingRuntimeTenant(t, db, tenant, prefix) })

	if _, _, _, err := currentBilling(svcCtx, ctx, tenant); err != nil {
		t.Fatalf("bootstrap free billing: %v", err)
	}
	if _, err := db.ExecCtx(ctx, `UPDATE t_tenant_entitlement SET builds_per_cycle=5 WHERE tenant_id=?`, tenant); err != nil {
		t.Fatal(err)
	}

	var accepted atomic.Int32
	var exhausted atomic.Int32
	keys := make(chan string, 20)
	var wait sync.WaitGroup
	for index := 0; index < 20; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			key := fmt.Sprintf("%s-build-%d", prefix, index)
			_, reserveErr := reserveQuota(ctx, svcCtx, &core.ReserveQuotaReq{
				Metric: core.QuotaMetric_QUOTA_METRIC_BUILD_COUNT, Quantity: 1,
				ResourceType: "acceptance_build", IdempotencyKey: key, TtlSeconds: 300,
			})
			if reserveErr == nil {
				accepted.Add(1)
				keys <- key
				return
			}
			if status.Code(reserveErr) == codes.ResourceExhausted {
				exhausted.Add(1)
				return
			}
			t.Errorf("unexpected reserve error: %v", reserveErr)
		}(index)
	}
	wait.Wait()
	close(keys)
	if accepted.Load() != 5 || exhausted.Load() != 15 {
		t.Fatalf("concurrent quota oversold: accepted=%d exhausted=%d", accepted.Load(), exhausted.Load())
	}
	for key := range keys {
		request := &core.QuotaReservationActionReq{Metric: core.QuotaMetric_QUOTA_METRIC_BUILD_COUNT,
			IdempotencyKey: key, UsageMetric: core.BillingUsageMetric_BILLING_USAGE_METRIC_BUILD_STARTED}
		if _, err := confirmQuota(ctx, svcCtx, request); err != nil {
			t.Fatal(err)
		}
		if _, err := confirmQuota(ctx, svcCtx, request); err != nil {
			t.Fatalf("duplicate quota confirmation is not idempotent: %v", err)
		}
	}
	var usageCount, thresholdCount int64
	if err := db.QueryRowCtx(ctx, &usageCount, `SELECT COUNT(*) FROM t_usage_ledger WHERE tenant_id=? AND metric='build.started'`, tenant); err != nil || usageCount != 5 {
		t.Fatalf("usage ledger mismatch: count=%d err=%v", usageCount, err)
	}
	if err := db.QueryRowCtx(ctx, &thresholdCount, `SELECT COUNT(*) FROM t_usage_threshold_notification WHERE tenant_id=?`, tenant); err != nil || thresholdCount != 3 {
		t.Fatalf("threshold notification mismatch: count=%d err=%v", thresholdCount, err)
	}

	var planID int64
	if err := db.QueryRowCtx(ctx, &planID, `SELECT id FROM t_billing_plan WHERE plan_code='pro' AND status=1 ORDER BY version DESC LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	paid := &core.ApplyBillingWebhookReq{Provider: "stripe", ProviderEventId: prefix + "-paid", EventType: "invoice.paid",
		EventCreatedAt: now, PayloadJson: `{"signed":true}`, TenantId: tenant, PlanId: planID,
		ExternalCustomerId: prefix + "-customer", ExternalSubscriptionId: prefix + "-subscription",
		ExternalInvoiceId: prefix + "-invoice", ExternalTransactionId: prefix + "-charge", Currency: "CNY",
		Amount: 19900, PeriodStart: now, PeriodEnd: now + 30*86400}
	if _, err := applyBillingWebhook(ctx, svcCtx, paid); err != nil {
		t.Fatalf("apply paid event: %v", err)
	}
	if _, err := applyBillingWebhook(ctx, svcCtx, paid); err != nil {
		t.Fatalf("duplicate paid event: %v", err)
	}
	olderDelete := &core.ApplyBillingWebhookReq{Provider: "stripe", ProviderEventId: prefix + "-old-delete",
		EventType: "customer.subscription.deleted", EventCreatedAt: now - 60, PayloadJson: `{"signed":true}`,
		TenantId: tenant, ExternalSubscriptionId: prefix + "-subscription"}
	if _, err := applyBillingWebhook(ctx, svcCtx, olderDelete); err != nil {
		t.Fatal(err)
	}
	var subscriptionStatus, invoiceCount int64
	if err := db.QueryRowCtx(ctx, &subscriptionStatus, `SELECT status FROM t_tenant_subscription WHERE tenant_id=?`, tenant); err != nil || subscriptionStatus != subscriptionActive {
		t.Fatalf("stale event regressed subscription: status=%d err=%v", subscriptionStatus, err)
	}
	if err := db.QueryRowCtx(ctx, &invoiceCount, `SELECT COUNT(*) FROM t_invoice WHERE tenant_id=?`, tenant); err != nil || invoiceCount != 1 {
		t.Fatalf("invoice idempotency failed: count=%d err=%v", invoiceCount, err)
	}
	refund := &core.ApplyBillingWebhookReq{Provider: "stripe", ProviderEventId: prefix + "-refund",
		EventType: "charge.refunded", EventCreatedAt: now + 1, PayloadJson: `{"signed":true}`,
		TenantId: tenant, ExternalInvoiceId: prefix + "-invoice", ExternalTransactionId: prefix + "-refund-txn",
		Currency: "CNY", Amount: 19900}
	if _, err := applyBillingWebhook(ctx, svcCtx, refund); err != nil {
		t.Fatal(err)
	}
	dispute := &core.ApplyBillingWebhookReq{Provider: "stripe", ProviderEventId: prefix + "-dispute",
		EventType: "charge.dispute.created", EventCreatedAt: now + 2, PayloadJson: `{"signed":true}`,
		TenantId: tenant, ExternalInvoiceId: prefix + "-invoice", ExternalTransactionId: prefix + "-dispute-txn",
		Currency: "CNY", Amount: 19900}
	if _, err := applyBillingWebhook(ctx, svcCtx, dispute); err != nil {
		t.Fatal(err)
	}
	var refunded, refundTransactions, disputeTransactions int64
	if err := db.QueryRowCtx(ctx, &refunded, `SELECT refunded_amount FROM t_invoice WHERE tenant_id=?`, tenant); err != nil || refunded != 19900 {
		t.Fatalf("refund mismatch: amount=%d err=%v", refunded, err)
	}
	_ = db.QueryRowCtx(ctx, &refundTransactions, `SELECT COUNT(*) FROM t_payment_transaction WHERE tenant_id=? AND transaction_type=2`, tenant)
	_ = db.QueryRowCtx(ctx, &disputeTransactions, `SELECT COUNT(*) FROM t_payment_transaction WHERE tenant_id=? AND transaction_type=3`, tenant)
	if refundTransactions != 1 || disputeTransactions != 1 {
		t.Fatalf("payment transaction mismatch: refunds=%d disputes=%d", refundTransactions, disputeTransactions)
	}
	var ciphertext string
	if err := db.QueryRowCtx(ctx, &ciphertext, `SELECT payload_ciphertext FROM t_billing_webhook_event WHERE provider_event_id=?`, paid.ProviderEventId); err != nil || strings.Contains(ciphertext, "signed") {
		t.Fatalf("webhook payload was not encrypted: err=%v", err)
	}
}

func cleanupBillingRuntimeTenant(t *testing.T, db sqlx.SqlConn, tenant int64, prefix string) {
	t.Helper()
	ctx := context.Background()
	queries := []string{
		`DELETE FROM t_payment_transaction WHERE tenant_id=?`, `DELETE FROM t_invoice_item WHERE tenant_id=?`,
		`DELETE FROM t_invoice WHERE tenant_id=?`, `DELETE FROM t_usage_threshold_notification WHERE tenant_id=?`,
		`DELETE FROM t_quota_reservation WHERE tenant_id=?`, `DELETE FROM t_usage_ledger WHERE tenant_id=?`,
		`DELETE FROM t_tenant_entitlement WHERE tenant_id=?`, `DELETE FROM t_tenant_subscription WHERE tenant_id=?`,
		`DELETE FROM t_billing_webhook_event WHERE provider_event_id LIKE ?`,
	}
	for _, query := range queries {
		argument := any(tenant)
		if strings.Contains(query, "provider_event_id") {
			argument = prefix + "%"
		}
		if _, err := db.ExecCtx(ctx, query, argument); err != nil {
			t.Logf("cleanup failed: %v", err)
		}
	}
}
