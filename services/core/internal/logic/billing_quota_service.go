package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func reserveQuota(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ReserveQuotaReq) (*core.QuotaReservationResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	metric, err := mapQuotaMetric(in.Metric)
	if err != nil {
		return nil, err
	}
	if in.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "quantity must be greater than zero")
	}
	if err := requireText(in.ResourceType, "resource_type", 64); err != nil {
		return nil, err
	}
	if err := requireText(in.IdempotencyKey, "idempotency_key", 191); err != nil {
		return nil, err
	}
	ttl := in.TtlSeconds
	if ttl <= 0 {
		ttl = 900
	}
	if ttl > 86400 {
		return nil, status.Error(codes.InvalidArgument, "ttl_seconds must not exceed 86400")
	}
	var reservation models.TQuotaReservation
	var used, reserved, limit int64
	var businessErr error
	now := billingNow()
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) (txErr error) {
		defer commitQuotaBusinessError(&businessErr, &txErr)
		subscription, entitlement, _, err := loadTenantBilling(txCtx, session, tenant, true)
		if err != nil {
			return err
		}
		if !subscriptionAllowsConsumption(subscription, now) || entitlement.Status != entitlementActive || now.Before(entitlement.ValidFrom) || !now.Before(entitlement.ValidUntil) {
			return status.Error(codes.FailedPrecondition, "SUBSCRIPTION_INACTIVE: current entitlement does not allow new consumption")
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_quota_reservation SET status=?,released_at=CURRENT_TIMESTAMP(3)
WHERE tenant_id=? AND status=? AND expires_at<=CURRENT_TIMESTAMP(3)`, quotaExpired, tenant, quotaReserved); err != nil {
			return billingInternalError("expire quota reservations", err)
		}
		err = session.QueryRowCtx(txCtx, &reservation, quotaReservationSelect+` WHERE tenant_id=? AND metric=? AND idempotency_key=? FOR UPDATE`, tenant, metric, strings.TrimSpace(in.IdempotencyKey))
		if err == nil {
			if reservation.Quantity != in.Quantity || reservation.ResourceType != strings.TrimSpace(in.ResourceType) || (in.ResourceId > 0 && reservation.ResourceId > 0 && reservation.ResourceId != in.ResourceId) {
				return status.Error(codes.AlreadyExists, "quota idempotency key is already used with different input")
			}
			used, reserved, limit, err = quotaSnapshot(txCtx, session, tenant, metric, subscription, entitlement)
			return err
		}
		if err != sqlx.ErrNotFound {
			return billingInternalError("load quota reservation", err)
		}
		used, reserved, limit, err = quotaSnapshot(txCtx, session, tenant, metric, subscription, entitlement)
		if err != nil {
			return err
		}
		if limit >= 0 && used+reserved+in.Quantity > limit {
			return quotaExceeded(metric, used, reserved, in.Quantity, limit)
		}
		result, err := session.ExecCtx(txCtx, `INSERT INTO t_quota_reservation
(tenant_id,metric,quantity,resource_type,resource_id,idempotency_key,period_key,status,expires_at)
VALUES (?,?,?,?,?,?,?,?,?)`, tenant, metric, in.Quantity, strings.TrimSpace(in.ResourceType), in.ResourceId,
			strings.TrimSpace(in.IdempotencyKey), periodKey(subscription.CurrentPeriodStart), quotaReserved, now.Add(time.Duration(ttl)*time.Second))
		if err != nil {
			return status.Errorf(codes.Aborted, "reserve quota failed; retry request: %v", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return billingInternalError("read quota reservation id", err)
		}
		if err := session.QueryRowCtx(txCtx, &reservation, quotaReservationSelect+` WHERE id=?`, id); err != nil {
			return billingInternalError("load created quota reservation", err)
		}
		reserved += in.Quantity
		return nil
	})
	if err != nil {
		return nil, err
	}
	if businessErr != nil {
		return nil, businessErr
	}
	return quotaReservationResponse(&reservation, used, reserved, limit), nil
}

func confirmQuota(ctx context.Context, svcCtx *svc.ServiceContext, in *core.QuotaReservationActionReq) (*core.QuotaReservationResp, error) {
	return mutateQuotaReservation(ctx, svcCtx, in, true)
}

func releaseQuota(ctx context.Context, svcCtx *svc.ServiceContext, in *core.QuotaReservationActionReq) (*core.QuotaReservationResp, error) {
	return mutateQuotaReservation(ctx, svcCtx, in, false)
}

func mutateQuotaReservation(ctx context.Context, svcCtx *svc.ServiceContext, in *core.QuotaReservationActionReq, confirm bool) (*core.QuotaReservationResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	metric, err := mapQuotaMetric(in.Metric)
	if err != nil {
		return nil, err
	}
	if err := requireText(in.IdempotencyKey, "idempotency_key", 191); err != nil {
		return nil, err
	}
	usageMetric := ""
	if confirm {
		usageMetric, err = mapUsageMetric(in.UsageMetric)
		if err != nil {
			return nil, err
		}
		if err := requireJSONOrEmpty(in.UsageMetadataJson, "usage_metadata_json"); err != nil {
			return nil, err
		}
	}
	var reservation models.TQuotaReservation
	var used, reserved, limit int64
	var businessErr error
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) (txErr error) {
		defer commitQuotaBusinessError(&businessErr, &txErr)
		subscription, entitlement, _, err := loadTenantBilling(txCtx, session, tenant, true)
		if err != nil {
			return err
		}
		if err := session.QueryRowCtx(txCtx, &reservation, quotaReservationSelect+` WHERE tenant_id=? AND metric=? AND idempotency_key=? FOR UPDATE`, tenant, metric, strings.TrimSpace(in.IdempotencyKey)); err != nil {
			if err == sqlx.ErrNotFound {
				return status.Error(codes.NotFound, "quota reservation not found")
			}
			return billingInternalError("lock quota reservation", err)
		}
		now := billingNow()
		if reservation.Status == quotaReserved && !now.Before(reservation.ExpiresAt) {
			reservation.Status = quotaExpired
			reservation.ReleasedAt = sql.NullTime{Time: now, Valid: true}
			if _, err := session.ExecCtx(txCtx, `UPDATE t_quota_reservation SET status=?,released_at=? WHERE id=?`, quotaExpired, now, reservation.Id); err != nil {
				return billingInternalError("expire quota reservation", err)
			}
		}
		if confirm {
			if reservation.Status == quotaConfirmed {
				used, reserved, limit, err = quotaSnapshot(txCtx, session, tenant, metric, subscription, entitlement)
				return err
			}
			if reservation.Status != quotaReserved {
				return status.Error(codes.FailedPrecondition, "quota reservation is no longer confirmable")
			}
			reservation.Status = quotaConfirmed
			reservation.ResourceId = in.ResourceId
			reservation.ConfirmedAt = sql.NullTime{Time: now, Valid: true}
			if _, err := session.ExecCtx(txCtx, `UPDATE t_quota_reservation SET status=?,resource_id=?,confirmed_at=? WHERE id=?`, quotaConfirmed, in.ResourceId, now, reservation.Id); err != nil {
				return billingInternalError("confirm quota reservation", err)
			}
			if err := insertUsageLedger(txCtx, session, tenant, usageMetric, reservation.Quantity,
				reservation.ResourceType, in.ResourceId, "quota:"+metric+":"+reservation.IdempotencyKey,
				now, in.UsageMetadataJson); err != nil {
				return err
			}
			if err := createUsageThresholdEvents(txCtx, session, tenant, usageMetric, subscription, entitlement); err != nil {
				return err
			}
		} else {
			if reservation.Status == quotaReleased || reservation.Status == quotaExpired {
				used, reserved, limit, err = quotaSnapshot(txCtx, session, tenant, metric, subscription, entitlement)
				return err
			}
			if reservation.Status == quotaConfirmed {
				return status.Error(codes.FailedPrecondition, "confirmed quota cannot be released; append a negative usage adjustment")
			}
			reservation.Status = quotaReleased
			reservation.ReleasedAt = sql.NullTime{Time: now, Valid: true}
			if _, err := session.ExecCtx(txCtx, `UPDATE t_quota_reservation SET status=?,released_at=? WHERE id=?`, quotaReleased, now, reservation.Id); err != nil {
				return billingInternalError("release quota reservation", err)
			}
		}
		used, reserved, limit, err = quotaSnapshot(txCtx, session, tenant, metric, subscription, entitlement)
		return err
	})
	if err != nil {
		return nil, err
	}
	if businessErr != nil {
		return nil, businessErr
	}
	return quotaReservationResponse(&reservation, used, reserved, limit), nil
}

func recordUsage(ctx context.Context, svcCtx *svc.ServiceContext, in *core.RecordUsageReq) (*core.RespBase, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	metric, err := mapUsageMetric(in.Metric)
	if err != nil {
		return nil, err
	}
	if in.Quantity == 0 {
		return nil, status.Error(codes.InvalidArgument, "quantity must not be zero")
	}
	if err := requireText(in.ResourceType, "resource_type", 64); err != nil {
		return nil, err
	}
	if err := requireText(in.IdempotencyKey, "idempotency_key", 191); err != nil {
		return nil, err
	}
	if err := requireJSONOrEmpty(in.MetadataJson, "metadata_json"); err != nil {
		return nil, err
	}
	occurred := billingNow()
	if in.OccurredAt > 0 {
		occurred = timeFromMillis(in.OccurredAt).UTC()
	}
	var businessErr error
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) (txErr error) {
		defer commitQuotaBusinessError(&businessErr, &txErr)
		subscription, entitlement, _, err := loadTenantBilling(txCtx, session, tenant, true)
		if err != nil {
			return err
		}
		if err := insertUsageLedger(txCtx, session, tenant, metric, in.Quantity, strings.TrimSpace(in.ResourceType),
			in.ResourceId, strings.TrimSpace(in.IdempotencyKey), occurred, in.MetadataJson); err != nil {
			return err
		}
		return createUsageThresholdEvents(txCtx, session, tenant, metric, subscription, entitlement)
	})
	if err != nil {
		return nil, err
	}
	if businessErr != nil {
		return nil, businessErr
	}
	return &core.RespBase{Base: okBase()}, nil
}

// commitQuotaBusinessError keeps expected quota and idempotency rejections out of
// sqlx's database breaker. Returning nil commits harmless cleanup performed before
// the rejection while genuine SQL failures still roll back and affect the breaker.
func commitQuotaBusinessError(target *error, txErr *error) {
	if txErr == nil || *txErr == nil {
		return
	}
	switch status.Code(*txErr) {
	case codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.PermissionDenied,
		codes.Unauthenticated, codes.ResourceExhausted, codes.FailedPrecondition:
		*target = *txErr
		*txErr = nil
	}
}

func insertUsageLedger(ctx context.Context, session sqlx.Session, tenant int64, metric string, quantity int64,
	resourceType string, resourceID int64, idempotencyKey string, occurred time.Time, metadata string,
) error {
	result, err := session.ExecCtx(ctx, `INSERT INTO t_usage_ledger
(tenant_id,metric,quantity,resource_type,resource_id,idempotency_key,occurred_at,period_key,metadata)
VALUES (?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`, tenant, metric, quantity,
		resourceType, resourceID, idempotencyKey, occurred, periodKey(occurred), nullString(metadata))
	if err != nil {
		return billingInternalError("record usage", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return billingInternalError("read usage ledger id", err)
	}
	var existing models.TUsageLedger
	if err := session.QueryRowCtx(ctx, &existing, `SELECT id,tenant_id,metric,quantity,resource_type,resource_id,
idempotency_key,occurred_at,period_key,metadata,create_time FROM t_usage_ledger WHERE id=?`, id); err != nil {
		return billingInternalError("load usage ledger", err)
	}
	if existing.TenantId != tenant || existing.Metric != metric || existing.Quantity != quantity ||
		existing.ResourceType != resourceType || existing.ResourceId != resourceID {
		return status.Error(codes.AlreadyExists, "usage idempotency key is already used with different input")
	}
	return nil
}

func quotaSnapshot(ctx context.Context, session sqlx.Session, tenant int64, metric string,
	subscription *models.TTenantSubscription, entitlement *models.TTenantEntitlement,
) (used int64, reserved int64, limit int64, err error) {
	used, err = usageForQuota(ctx, session, tenant, metric, subscription)
	if err != nil {
		return 0, 0, 0, err
	}
	if err = session.QueryRowCtx(ctx, &reserved, `SELECT COALESCE(SUM(quantity),0) FROM t_quota_reservation
WHERE tenant_id=? AND metric=? AND period_key=? AND status=? AND expires_at>CURRENT_TIMESTAMP(3)`,
		tenant, metric, periodKey(subscription.CurrentPeriodStart), quotaReserved); err != nil {
		return 0, 0, 0, billingInternalError("sum quota reservations", err)
	}
	return used, reserved, quotaLimit(entitlement, metric), nil
}

func usageForQuota(ctx context.Context, session sqlx.Session, tenant int64, metric string, subscription *models.TTenantSubscription) (int64, error) {
	var used int64
	var query string
	var args []any
	switch metric {
	case "build.count":
		query = `SELECT COALESCE(SUM(quantity),0) FROM t_usage_ledger WHERE tenant_id=? AND metric='build.started' AND occurred_at>=? AND occurred_at<?`
		args = []any{tenant, subscription.CurrentPeriodStart, subscription.CurrentPeriodEnd}
	case "storage.bytes":
		query = `SELECT COALESCE(SUM(quantity),0) FROM t_usage_ledger WHERE tenant_id=? AND metric IN ('storage.source_bytes','storage.artifact_bytes','storage.log_bytes')`
		args = []any{tenant}
	case "team.seats":
		query = `SELECT COALESCE(SUM(quantity),0) FROM t_usage_ledger WHERE tenant_id=? AND metric='team.active_seats'`
		args = []any{tenant}
	default:
		return 0, status.Error(codes.InvalidArgument, "quota metric is invalid")
	}
	if err := session.QueryRowCtx(ctx, &used, query, args...); err != nil {
		return 0, billingInternalError("sum quota usage", err)
	}
	if used < 0 {
		used = 0
	}
	return used, nil
}

func quotaReservationResponse(item *models.TQuotaReservation, used, reserved, limit int64) *core.QuotaReservationResp {
	return &core.QuotaReservationResp{Base: okBase(), Data: &core.QuotaReservation{
		Id: item.Id, TenantId: item.TenantId, Metric: parseQuotaMetric(item.Metric), Quantity: item.Quantity,
		ResourceType: item.ResourceType, ResourceId: item.ResourceId, IdempotencyKey: item.IdempotencyKey,
		PeriodKey: item.PeriodKey, Status: core.QuotaReservationStatus(item.Status), ExpiresAt: millis(item.ExpiresAt),
		UsedQuantity: used, ReservedQuantity: reserved, LimitQuantity: limit,
	}}
}

func createUsageThresholdEvents(ctx context.Context, session sqlx.Session, tenant int64, usageMetric string,
	subscription *models.TTenantSubscription, entitlement *models.TTenantEntitlement,
) error {
	quotaMetric := ""
	switch usageMetric {
	case "build.started":
		quotaMetric = "build.count"
	case "storage.source_bytes", "storage.artifact_bytes", "storage.log_bytes":
		quotaMetric = "storage.bytes"
	case "team.active_seats":
		quotaMetric = "team.seats"
	default:
		return nil
	}
	used, err := usageForQuota(ctx, session, tenant, quotaMetric, subscription)
	if err != nil {
		return err
	}
	limit := quotaLimit(entitlement, quotaMetric)
	if limit <= 0 {
		return nil
	}
	percent := used * 100 / limit
	for _, threshold := range []int64{70, 90, 100} {
		if percent < threshold {
			continue
		}
		result, err := session.ExecCtx(ctx, `INSERT IGNORE INTO t_usage_threshold_notification
(tenant_id,metric,period_key,threshold_percent,usage_quantity,limit_quantity,status,outbox_event_id)
VALUES (?,?,?,?,?,?,1,0)`, tenant, quotaMetric, periodKey(subscription.CurrentPeriodStart), threshold, used, limit)
		if err != nil {
			return billingInternalError("create usage threshold notification", err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			continue
		}
		notificationID, _ := result.LastInsertId()
		eventType := "quota.warning"
		if threshold >= 100 {
			eventType = "quota.exceeded"
		}
		outboxID, _, err := insertOutboxEvent(ctx, session, tenant, eventType, "quota", notificationID,
			map[string]any{"metric": quotaMetric, "periodKey": periodKey(subscription.CurrentPeriodStart),
				"thresholdPercent": threshold, "used": used, "limit": limit})
		if err != nil {
			return billingInternalError("create quota outbox event", err)
		}
		if _, err := session.ExecCtx(ctx, `UPDATE t_usage_threshold_notification SET status=2,outbox_event_id=? WHERE id=?`, outboxID, notificationID); err != nil {
			return billingInternalError("link quota notification outbox", err)
		}
	}
	return nil
}

func getBillingUsage(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BillingUsageReq) (*core.BillingUsageResp, error) {
	if in == nil {
		in = &core.BillingUsageReq{}
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	subscription, entitlement, _, err := currentBilling(svcCtx, ctx, tenant)
	if err != nil {
		return nil, err
	}
	requestedPeriod := strings.TrimSpace(in.PeriodKey)
	if requestedPeriod == "" {
		requestedPeriod = periodKey(subscription.CurrentPeriodStart)
	}
	if len(requestedPeriod) != 7 || requestedPeriod[4] != '-' {
		return nil, status.Error(codes.InvalidArgument, "period_key must use YYYY-MM")
	}
	var rows []struct {
		Metric string `db:"metric"`
		Used   int64  `db:"used"`
	}
	if err := svcCtx.DB.QueryRowsCtx(ctx, &rows, `SELECT metric,COALESCE(SUM(quantity),0) used FROM t_usage_ledger
WHERE tenant_id=? AND (period_key=? OR metric IN ('storage.source_bytes','storage.artifact_bytes','storage.log_bytes','team.active_seats'))
GROUP BY metric`, tenant, requestedPeriod); err != nil {
		return nil, billingInternalError("summarize billing usage", err)
	}
	usage := make(map[string]int64, len(rows))
	for _, row := range rows {
		usage[row.Metric] = row.Used
	}
	reservations := map[string]int64{}
	var reservedRows []struct {
		Metric   string `db:"metric"`
		Reserved int64  `db:"reserved"`
	}
	if err := svcCtx.DB.QueryRowsCtx(ctx, &reservedRows, `SELECT metric,COALESCE(SUM(quantity),0) reserved FROM t_quota_reservation
WHERE tenant_id=? AND period_key=? AND status=? AND expires_at>CURRENT_TIMESTAMP(3) GROUP BY metric`, tenant, requestedPeriod, quotaReserved); err != nil {
		return nil, billingInternalError("summarize quota reservations", err)
	}
	for _, row := range reservedRows {
		reservations[row.Metric] = row.Reserved
	}
	metrics := []core.BillingUsageMetric{
		core.BillingUsageMetric_BILLING_USAGE_METRIC_BUILD_STARTED,
		core.BillingUsageMetric_BILLING_USAGE_METRIC_BUILD_SUCCEEDED,
		core.BillingUsageMetric_BILLING_USAGE_METRIC_BUILD_COMPUTE_SECONDS,
		core.BillingUsageMetric_BILLING_USAGE_METRIC_STORAGE_SOURCE_BYTES,
		core.BillingUsageMetric_BILLING_USAGE_METRIC_STORAGE_ARTIFACT_BYTES,
		core.BillingUsageMetric_BILLING_USAGE_METRIC_STORAGE_LOG_BYTES,
		core.BillingUsageMetric_BILLING_USAGE_METRIC_API_REQUESTS,
		core.BillingUsageMetric_BILLING_USAGE_METRIC_TEAM_ACTIVE_SEATS,
	}
	items := make([]*core.UsageMetricSummary, 0, len(metrics))
	for _, metric := range metrics {
		name, _ := mapUsageMetric(metric)
		limit := int64(-1)
		reserved := int64(0)
		switch metric {
		case core.BillingUsageMetric_BILLING_USAGE_METRIC_BUILD_STARTED:
			limit, reserved = entitlement.BuildsPerCycle, reservations["build.count"]
		case core.BillingUsageMetric_BILLING_USAGE_METRIC_STORAGE_SOURCE_BYTES,
			core.BillingUsageMetric_BILLING_USAGE_METRIC_STORAGE_ARTIFACT_BYTES,
			core.BillingUsageMetric_BILLING_USAGE_METRIC_STORAGE_LOG_BYTES:
			limit, reserved = entitlement.StorageBytes, reservations["storage.bytes"]
		case core.BillingUsageMetric_BILLING_USAGE_METRIC_TEAM_ACTIVE_SEATS:
			limit, reserved = entitlement.TeamSeats, reservations["team.seats"]
		case core.BillingUsageMetric_BILLING_USAGE_METRIC_API_REQUESTS:
			limit = entitlement.ApiRateLimit
		}
		used := usage[name]
		percent := int32(0)
		if limit > 0 {
			percentValue := (used + reserved) * 100 / limit
			if percentValue > 2147483647 {
				percentValue = 2147483647
			}
			percent = int32(percentValue)
		}
		items = append(items, &core.UsageMetricSummary{Metric: metric, UsedQuantity: used,
			ReservedQuantity: reserved, LimitQuantity: limit, UsagePercent: percent})
	}
	return &core.BillingUsageResp{Base: okBase(), PeriodKey: requestedPeriod, Data: items}, nil
}

func billingUsageMetadata(value map[string]any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func storageUsageMetric(objectType int64) core.BillingUsageMetric {
	switch core.StorageObjectType(objectType) {
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK, core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_CACHE:
		return core.BillingUsageMetric_BILLING_USAGE_METRIC_STORAGE_ARTIFACT_BYTES
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_LOG:
		return core.BillingUsageMetric_BILLING_USAGE_METRIC_STORAGE_LOG_BYTES
	default:
		return core.BillingUsageMetric_BILLING_USAGE_METRIC_STORAGE_SOURCE_BYTES
	}
}

func storageQuotaKey(objectKey string) string {
	return fmt.Sprintf("storage:%s", strings.TrimSpace(objectKey))
}

// reserveQuotaInSession applies an entitlement check and writes the reservation in
// the caller's transaction. Business writes and quota reservations therefore
// either commit together or roll back together.
func reserveQuotaInSession(ctx context.Context, session sqlx.Session, tenant int64, metric string, quantity int64,
	resourceType string, resourceID int64, idempotencyKey string, ttl time.Duration,
) (*models.TQuotaReservation, error) {
	if quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "quota quantity must be greater than zero")
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	now := billingNow()
	subscription, entitlement, _, err := loadTenantBilling(ctx, session, tenant, true)
	if err != nil {
		return nil, err
	}
	if !subscriptionAllowsConsumption(subscription, now) || entitlement.Status != entitlementActive ||
		now.Before(entitlement.ValidFrom) || !now.Before(entitlement.ValidUntil) {
		return nil, status.Error(codes.FailedPrecondition, "SUBSCRIPTION_INACTIVE: current entitlement does not allow new consumption")
	}
	if metric == "storage.bytes" && entitlement.MaxUploadBytes >= 0 && quantity > entitlement.MaxUploadBytes {
		return nil, status.Errorf(codes.ResourceExhausted,
			"UPLOAD_TOO_LARGE: upload size %d exceeds entitlement maximum %d", quantity, entitlement.MaxUploadBytes)
	}
	if _, err := session.ExecCtx(ctx, `UPDATE t_quota_reservation SET status=?,released_at=CURRENT_TIMESTAMP(3)
WHERE tenant_id=? AND status=? AND expires_at<=CURRENT_TIMESTAMP(3)`, quotaExpired, tenant, quotaReserved); err != nil {
		return nil, billingInternalError("expire quota reservations", err)
	}
	var existing models.TQuotaReservation
	err = session.QueryRowCtx(ctx, &existing, quotaReservationSelect+` WHERE tenant_id=? AND metric=? AND idempotency_key=? FOR UPDATE`,
		tenant, metric, idempotencyKey)
	if err == nil {
		if existing.Quantity != quantity || existing.ResourceType != resourceType ||
			(resourceID > 0 && existing.ResourceId > 0 && existing.ResourceId != resourceID) {
			return nil, status.Error(codes.AlreadyExists, "quota idempotency key is already used with different input")
		}
		return &existing, nil
	}
	if err != sqlx.ErrNotFound {
		return nil, billingInternalError("load quota reservation", err)
	}
	used, reserved, limit, err := quotaSnapshot(ctx, session, tenant, metric, subscription, entitlement)
	if err != nil {
		return nil, err
	}
	if limit >= 0 && used+reserved+quantity > limit {
		return nil, quotaExceeded(metric, used, reserved, quantity, limit)
	}
	result, err := session.ExecCtx(ctx, `INSERT INTO t_quota_reservation
(tenant_id,metric,quantity,resource_type,resource_id,idempotency_key,period_key,status,expires_at)
VALUES (?,?,?,?,?,?,?,?,?)`, tenant, metric, quantity, resourceType, resourceID, idempotencyKey,
		periodKey(subscription.CurrentPeriodStart), quotaReserved, now.Add(ttl))
	if err != nil {
		return nil, status.Errorf(codes.Aborted, "reserve quota failed; retry request: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, billingInternalError("read quota reservation id", err)
	}
	var created models.TQuotaReservation
	if err := session.QueryRowCtx(ctx, &created, quotaReservationSelect+` WHERE id=?`, id); err != nil {
		return nil, billingInternalError("load created quota reservation", err)
	}
	return &created, nil
}

func confirmQuotaInSession(ctx context.Context, session sqlx.Session, tenant int64, metric, idempotencyKey,
	usageMetric string, resourceID int64, metadata string,
) error {
	var reservation models.TQuotaReservation
	if err := session.QueryRowCtx(ctx, &reservation, quotaReservationSelect+` WHERE tenant_id=? AND metric=? AND idempotency_key=? FOR UPDATE`,
		tenant, metric, idempotencyKey); err != nil {
		if err == sqlx.ErrNotFound {
			return status.Error(codes.FailedPrecondition, "quota reservation not found")
		}
		return billingInternalError("lock quota reservation", err)
	}
	if reservation.Status == quotaConfirmed {
		return nil
	}
	if reservation.Status != quotaReserved || !billingNow().Before(reservation.ExpiresAt) {
		return status.Error(codes.FailedPrecondition, "quota reservation is no longer confirmable")
	}
	now := billingNow()
	if _, err := session.ExecCtx(ctx, `UPDATE t_quota_reservation SET status=?,resource_id=?,confirmed_at=? WHERE id=?`,
		quotaConfirmed, resourceID, now, reservation.Id); err != nil {
		return billingInternalError("confirm quota reservation", err)
	}
	if err := insertUsageLedger(ctx, session, tenant, usageMetric, reservation.Quantity, reservation.ResourceType,
		resourceID, "quota:"+metric+":"+reservation.IdempotencyKey, now, metadata); err != nil {
		return err
	}
	subscription, entitlement, _, err := loadTenantBilling(ctx, session, tenant, false)
	if err != nil {
		return err
	}
	return createUsageThresholdEvents(ctx, session, tenant, usageMetric, subscription, entitlement)
}

func releaseQuotaInSession(ctx context.Context, session sqlx.Session, tenant int64, metric, idempotencyKey string) error {
	var reservation models.TQuotaReservation
	if err := session.QueryRowCtx(ctx, &reservation, quotaReservationSelect+` WHERE tenant_id=? AND metric=? AND idempotency_key=? FOR UPDATE`,
		tenant, metric, idempotencyKey); err != nil {
		if err == sqlx.ErrNotFound {
			return nil
		}
		return billingInternalError("lock quota reservation", err)
	}
	if reservation.Status == quotaReleased || reservation.Status == quotaExpired {
		return nil
	}
	if reservation.Status == quotaConfirmed {
		return status.Error(codes.FailedPrecondition, "confirmed quota cannot be released")
	}
	_, err := session.ExecCtx(ctx, `UPDATE t_quota_reservation SET status=?,released_at=CURRENT_TIMESTAMP(3) WHERE id=?`,
		quotaReleased, reservation.Id)
	return err
}

func adjustUsageInSession(ctx context.Context, session sqlx.Session, tenant int64, metric string, quantity int64,
	resourceType string, resourceID int64, idempotencyKey string, metadata map[string]any,
) error {
	if quantity == 0 {
		return nil
	}
	return insertUsageLedger(ctx, session, tenant, metric, quantity, resourceType, resourceID, idempotencyKey,
		billingNow(), billingUsageMetadata(metadata))
}
