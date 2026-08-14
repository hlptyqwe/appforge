package system

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"appforge/admin-api/internal/svc"
	"appforge/proto/core"
)

func newSeatQuotaKey(tenantID int64, operation string) string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err == nil {
		return fmt.Sprintf("seat:%d:%s:%s", tenantID, operation, hex.EncodeToString(value))
	}
	return fmt.Sprintf("seat:%d:%s:%d", tenantID, operation, time.Now().UnixNano())
}

func reserveSeat(ctx context.Context, svcCtx *svc.ServiceContext, tenantID int64, key string) error {
	_, err := svcCtx.CoreCli.ReserveQuota(ctx, &core.ReserveQuotaReq{
		TenantId: tenantID, Metric: core.QuotaMetric_QUOTA_METRIC_TEAM_SEATS, Quantity: 1,
		ResourceType: "system_user", IdempotencyKey: key, TtlSeconds: 300,
	})
	return err
}

func confirmSeat(ctx context.Context, svcCtx *svc.ServiceContext, tenantID, userID int64, key string) error {
	_, err := svcCtx.CoreCli.ConfirmQuota(ctx, &core.QuotaReservationActionReq{
		TenantId: tenantID, Metric: core.QuotaMetric_QUOTA_METRIC_TEAM_SEATS,
		IdempotencyKey: key, ResourceId: userID,
		UsageMetric:       core.BillingUsageMetric_BILLING_USAGE_METRIC_TEAM_ACTIVE_SEATS,
		UsageMetadataJson: `{"source":"system-user"}`,
	})
	return err
}

func releaseSeat(ctx context.Context, svcCtx *svc.ServiceContext, tenantID int64, key string) {
	_, _ = svcCtx.CoreCli.ReleaseQuota(ctx, &core.QuotaReservationActionReq{
		TenantId: tenantID, Metric: core.QuotaMetric_QUOTA_METRIC_TEAM_SEATS, IdempotencyKey: key,
	})
}

func removeSeatUsage(ctx context.Context, svcCtx *svc.ServiceContext, tenantID, userID int64, key, reason string) error {
	_, err := svcCtx.CoreCli.RecordUsage(ctx, &core.RecordUsageReq{
		TenantId: tenantID, Metric: core.BillingUsageMetric_BILLING_USAGE_METRIC_TEAM_ACTIVE_SEATS,
		Quantity: -1, ResourceType: "system_user", ResourceId: userID, IdempotencyKey: key,
		MetadataJson: fmt.Sprintf(`{"reason":%q}`, reason),
	})
	return err
}
