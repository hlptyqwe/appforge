package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"appforge/common/utils"
	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	billingPlanActive  = int64(core.BillingPlanStatus_BILLING_PLAN_STATUS_ACTIVE)
	billingPlanRetired = int64(core.BillingPlanStatus_BILLING_PLAN_STATUS_RETIRED)

	subscriptionActive   = int64(core.TenantSubscriptionStatus_TENANT_SUBSCRIPTION_STATUS_ACTIVE)
	subscriptionPastDue  = int64(core.TenantSubscriptionStatus_TENANT_SUBSCRIPTION_STATUS_PAST_DUE)
	subscriptionGrace    = int64(core.TenantSubscriptionStatus_TENANT_SUBSCRIPTION_STATUS_GRACE)
	subscriptionPaused   = int64(core.TenantSubscriptionStatus_TENANT_SUBSCRIPTION_STATUS_PAUSED)
	subscriptionCanceled = int64(core.TenantSubscriptionStatus_TENANT_SUBSCRIPTION_STATUS_CANCELED)
	subscriptionPending  = int64(core.TenantSubscriptionStatus_TENANT_SUBSCRIPTION_STATUS_PENDING)

	entitlementActive = int64(core.TenantEntitlementStatus_TENANT_ENTITLEMENT_STATUS_ACTIVE)
	entitlementPaused = int64(core.TenantEntitlementStatus_TENANT_ENTITLEMENT_STATUS_PAUSED)

	quotaReserved  = int64(core.QuotaReservationStatus_QUOTA_RESERVATION_STATUS_RESERVED)
	quotaConfirmed = int64(core.QuotaReservationStatus_QUOTA_RESERVATION_STATUS_CONFIRMED)
	quotaReleased  = int64(core.QuotaReservationStatus_QUOTA_RESERVATION_STATUS_RELEASED)
	quotaExpired   = int64(core.QuotaReservationStatus_QUOTA_RESERVATION_STATUS_EXPIRED)
)

var billingPlanCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)
var billingCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

const billingPlanSelect = `SELECT id,plan_code,plan_name,billing_cycle,price_amount,currency,feature_json,
builds_per_cycle,max_build_concurrency,storage_bytes,max_upload_bytes,team_seats,api_rate_limit,
charge_failed_build,charge_cache_hit,charge_retry_build,status,version,create_time,update_time
FROM t_billing_plan`

const tenantSubscriptionSelect = `SELECT id,tenant_id,plan_id,plan_version,status,source,
external_customer_id,external_subscription_id,current_period_start,current_period_end,cancel_at_period_end,
grace_until,pending_plan_id,pending_plan_version,last_provider_event_at,create_time,update_time
FROM t_tenant_subscription`

const tenantEntitlementSelect = `SELECT id,tenant_id,source_type,source_id,plan_id,plan_version,
builds_per_cycle,max_build_concurrency,storage_bytes,max_upload_bytes,team_seats,api_rate_limit,
charge_failed_build,charge_cache_hit,charge_retry_build,override_json,valid_from,valid_until,status,revision,
create_time,update_time FROM t_tenant_entitlement`

const quotaReservationSelect = `SELECT id,tenant_id,metric,quantity,resource_type,resource_id,idempotency_key,
period_key,status,expires_at,confirmed_at,released_at,create_time,update_time FROM t_quota_reservation`

const invoiceSelect = `SELECT id,tenant_id,subscription_id,invoice_no,external_invoice_id,status,currency,
subtotal_amount,discount_amount,tax_amount,total_amount,paid_amount,refunded_amount,period_start,period_end,
due_at,paid_at,create_time,update_time FROM t_invoice`

const invoiceItemSelect = `SELECT id,tenant_id,invoice_id,line_key,item_type,description,metric,quantity,
unit_amount,amount,metadata,create_time FROM t_invoice_item`

func billingTargetTenant(ctx context.Context, requested int64) (int64, error) {
	if current, err := tenantID(ctx); err == nil && current > 0 {
		if requested == 0 || requested == current {
			return current, nil
		}
		return 0, status.Error(codes.PermissionDenied, "cross-tenant billing access is denied")
	}
	if requested <= 0 {
		return 0, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	userType, err := utils.GetUserTypeFromMd(ctx)
	if err != nil || userType != utils.SysUserTypeSystemAdmin {
		return 0, status.Error(codes.PermissionDenied, "system administrator is required")
	}
	return requested, nil
}

func validateSystemBillingActor(ctx context.Context) error {
	userType, err := utils.GetUserTypeFromMd(ctx)
	if err == nil && userType == utils.SysUserTypeSystemAdmin {
		return nil
	}
	return status.Error(codes.PermissionDenied, "system administrator is required")
}

func normalizeBillingPage(page, pageSize int32) (int64, int64) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return int64(pageSize), int64(page-1) * int64(pageSize)
}

func validateBillingPlanInput(in *core.CreateBillingPlanReq) error {
	if in == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	in.PlanCode = strings.ToLower(strings.TrimSpace(in.PlanCode))
	in.PlanName = strings.TrimSpace(in.PlanName)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if !billingPlanCodePattern.MatchString(in.PlanCode) {
		return status.Error(codes.InvalidArgument, "plan_code must match ^[a-z][a-z0-9_-]{1,63}$")
	}
	if err := requireText(in.PlanName, "plan_name", 128); err != nil {
		return err
	}
	if in.BillingCycle != core.BillingCycle_BILLING_CYCLE_MONTHLY && in.BillingCycle != core.BillingCycle_BILLING_CYCLE_YEARLY {
		return status.Error(codes.InvalidArgument, "billing_cycle is invalid")
	}
	if in.PriceAmount < 0 {
		return status.Error(codes.InvalidArgument, "price_amount must not be negative")
	}
	if !billingCurrencyPattern.MatchString(in.Currency) {
		return status.Error(codes.InvalidArgument, "currency must be a three-letter ISO code")
	}
	if strings.TrimSpace(in.FeatureJson) == "" {
		in.FeatureJson = "{}"
	}
	var features map[string]any
	if json.Unmarshal([]byte(in.FeatureJson), &features) != nil {
		return status.Error(codes.InvalidArgument, "feature_json must be a JSON object")
	}
	for _, item := range []struct {
		value int64
		field string
	}{
		{in.BuildsPerCycle, "builds_per_cycle"},
		{int64(in.MaxBuildConcurrency), "max_build_concurrency"},
		{in.StorageBytes, "storage_bytes"},
		{in.MaxUploadBytes, "max_upload_bytes"},
		{int64(in.TeamSeats), "team_seats"},
		{int64(in.ApiRateLimit), "api_rate_limit"},
	} {
		if item.value < -1 {
			return status.Errorf(codes.InvalidArgument, "%s must be -1 or greater", item.field)
		}
	}
	return nil
}

func mapBillingPlan(item *models.TBillingPlan) *core.BillingPlan {
	if item == nil {
		return nil
	}
	return &core.BillingPlan{
		Id: item.Id, PlanCode: item.PlanCode, PlanName: item.PlanName,
		BillingCycle: core.BillingCycle(item.BillingCycle), PriceAmount: item.PriceAmount,
		Currency: item.Currency, FeatureJson: item.FeatureJson, BuildsPerCycle: item.BuildsPerCycle,
		MaxBuildConcurrency: int32(item.MaxBuildConcurrency), StorageBytes: item.StorageBytes,
		MaxUploadBytes: item.MaxUploadBytes, TeamSeats: int32(item.TeamSeats),
		ApiRateLimit: int32(item.ApiRateLimit), ChargeFailedBuild: item.ChargeFailedBuild == 1,
		ChargeCacheHit: item.ChargeCacheHit == 1, ChargeRetryBuild: item.ChargeRetryBuild == 1,
		Status: core.BillingPlanStatus(item.Status), Version: int32(item.Version),
		CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime),
	}
}

func mapTenantSubscription(item *models.TTenantSubscription) *core.TenantSubscription {
	if item == nil {
		return nil
	}
	return &core.TenantSubscription{
		Id: item.Id, TenantId: item.TenantId, PlanId: item.PlanId, PlanVersion: int32(item.PlanVersion),
		Status: core.TenantSubscriptionStatus(item.Status), Source: core.TenantSubscriptionSource(item.Source),
		ExternalCustomerId:     stringValue(item.ExternalCustomerId),
		ExternalSubscriptionId: stringValue(item.ExternalSubscriptionId),
		CurrentPeriodStart:     millis(item.CurrentPeriodStart), CurrentPeriodEnd: millis(item.CurrentPeriodEnd),
		CancelAtPeriodEnd: item.CancelAtPeriodEnd == 1, GraceUntil: timeValue(item.GraceUntil),
		PendingPlanId: item.PendingPlanId, PendingPlanVersion: int32(item.PendingPlanVersion),
		CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime),
	}
}

func mapTenantEntitlement(item *models.TTenantEntitlement) *core.TenantEntitlement {
	if item == nil {
		return nil
	}
	return &core.TenantEntitlement{
		Id: item.Id, TenantId: item.TenantId, PlanId: item.PlanId, PlanVersion: int32(item.PlanVersion),
		BuildsPerCycle: item.BuildsPerCycle, MaxBuildConcurrency: int32(item.MaxBuildConcurrency),
		StorageBytes: item.StorageBytes, MaxUploadBytes: item.MaxUploadBytes,
		TeamSeats: int32(item.TeamSeats), ApiRateLimit: int32(item.ApiRateLimit),
		ChargeFailedBuild: item.ChargeFailedBuild == 1, ChargeCacheHit: item.ChargeCacheHit == 1,
		ChargeRetryBuild: item.ChargeRetryBuild == 1, OverrideJson: stringValue(item.OverrideJson),
		ValidFrom: millis(item.ValidFrom), ValidUntil: millis(item.ValidUntil),
		Status: core.TenantEntitlementStatus(item.Status), Revision: item.Revision,
	}
}

func mapQuotaMetric(value core.QuotaMetric) (string, error) {
	switch value {
	case core.QuotaMetric_QUOTA_METRIC_BUILD_COUNT:
		return "build.count", nil
	case core.QuotaMetric_QUOTA_METRIC_STORAGE_BYTES:
		return "storage.bytes", nil
	case core.QuotaMetric_QUOTA_METRIC_TEAM_SEATS:
		return "team.seats", nil
	default:
		return "", status.Error(codes.InvalidArgument, "quota metric is invalid")
	}
}

func parseQuotaMetric(value string) core.QuotaMetric {
	switch value {
	case "build.count":
		return core.QuotaMetric_QUOTA_METRIC_BUILD_COUNT
	case "storage.bytes":
		return core.QuotaMetric_QUOTA_METRIC_STORAGE_BYTES
	case "team.seats":
		return core.QuotaMetric_QUOTA_METRIC_TEAM_SEATS
	default:
		return core.QuotaMetric_QUOTA_METRIC_UNKNOWN
	}
}

func mapUsageMetric(value core.BillingUsageMetric) (string, error) {
	switch value {
	case core.BillingUsageMetric_BILLING_USAGE_METRIC_BUILD_STARTED:
		return "build.started", nil
	case core.BillingUsageMetric_BILLING_USAGE_METRIC_BUILD_SUCCEEDED:
		return "build.succeeded", nil
	case core.BillingUsageMetric_BILLING_USAGE_METRIC_BUILD_COMPUTE_SECONDS:
		return "build.compute_seconds", nil
	case core.BillingUsageMetric_BILLING_USAGE_METRIC_STORAGE_SOURCE_BYTES:
		return "storage.source_bytes", nil
	case core.BillingUsageMetric_BILLING_USAGE_METRIC_STORAGE_ARTIFACT_BYTES:
		return "storage.artifact_bytes", nil
	case core.BillingUsageMetric_BILLING_USAGE_METRIC_STORAGE_LOG_BYTES:
		return "storage.log_bytes", nil
	case core.BillingUsageMetric_BILLING_USAGE_METRIC_API_REQUESTS:
		return "api.requests", nil
	case core.BillingUsageMetric_BILLING_USAGE_METRIC_TEAM_ACTIVE_SEATS:
		return "team.active_seats", nil
	default:
		return "", status.Error(codes.InvalidArgument, "usage metric is invalid")
	}
}

func parseUsageMetric(value string) core.BillingUsageMetric {
	for metric := core.BillingUsageMetric_BILLING_USAGE_METRIC_BUILD_STARTED; metric <= core.BillingUsageMetric_BILLING_USAGE_METRIC_TEAM_ACTIVE_SEATS; metric++ {
		if name, err := mapUsageMetric(metric); err == nil && name == value {
			return metric
		}
	}
	return core.BillingUsageMetric_BILLING_USAGE_METRIC_UNKNOWN
}

func quotaLimit(entitlement *models.TTenantEntitlement, metric string) int64 {
	if entitlement == nil {
		return 0
	}
	switch metric {
	case "build.count":
		return entitlement.BuildsPerCycle
	case "storage.bytes":
		return entitlement.StorageBytes
	case "team.seats":
		return entitlement.TeamSeats
	default:
		return 0
	}
}

func periodKey(value time.Time) string {
	return value.UTC().Format("2006-01")
}

func subscriptionAllowsConsumption(item *models.TTenantSubscription, now time.Time) bool {
	if item == nil {
		return false
	}
	if item.Status == subscriptionActive && now.Before(item.CurrentPeriodEnd) {
		return true
	}
	return item.Status == subscriptionGrace && item.GraceUntil.Valid && now.Before(item.GraceUntil.Time)
}

func loadTenantBilling(ctx context.Context, session sqlx.Session, tenant int64, forUpdate bool) (*models.TTenantSubscription, *models.TTenantEntitlement, *models.TBillingPlan, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	var subscription models.TTenantSubscription
	if err := session.QueryRowCtx(ctx, &subscription, tenantSubscriptionSelect+` WHERE tenant_id=?`+suffix, tenant); err != nil {
		if err == sqlx.ErrNotFound {
			if bootstrapErr := bootstrapFreeTenantBilling(ctx, session, tenant); bootstrapErr != nil {
				return nil, nil, nil, bootstrapErr
			}
			if retryErr := session.QueryRowCtx(ctx, &subscription, tenantSubscriptionSelect+` WHERE tenant_id=?`+suffix, tenant); retryErr != nil {
				return nil, nil, nil, status.Errorf(codes.Internal, "load bootstrapped subscription failed: %v", retryErr)
			}
		} else {
			return nil, nil, nil, status.Errorf(codes.Internal, "load subscription failed: %v", err)
		}
	}
	var entitlement models.TTenantEntitlement
	if err := session.QueryRowCtx(ctx, &entitlement, tenantEntitlementSelect+` WHERE tenant_id=?`+suffix, tenant); err != nil {
		return nil, nil, nil, status.Errorf(codes.Internal, "load entitlement failed: %v", err)
	}
	var plan models.TBillingPlan
	if err := session.QueryRowCtx(ctx, &plan, billingPlanSelect+` WHERE id=?`, subscription.PlanId); err != nil {
		return nil, nil, nil, status.Errorf(codes.Internal, "load subscribed plan failed: %v", err)
	}
	return &subscription, &entitlement, &plan, nil
}

func bootstrapFreeTenantBilling(ctx context.Context, session sqlx.Session, tenant int64) error {
	var plan models.TBillingPlan
	if err := session.QueryRowCtx(ctx, &plan, billingPlanSelect+` WHERE plan_code='free' AND status=? ORDER BY version DESC LIMIT 1`, billingPlanActive); err != nil {
		if err == sqlx.ErrNotFound {
			return status.Error(codes.FailedPrecondition, "SUBSCRIPTION_MISSING: free billing plan is not configured")
		}
		return billingInternalError("load free billing plan", err)
	}
	start := billingNow()
	end := billingPeriodForCycle(start, plan.BillingCycle)
	if _, err := session.ExecCtx(ctx, `INSERT IGNORE INTO t_tenant_subscription
(tenant_id,plan_id,plan_version,status,source,current_period_start,current_period_end,cancel_at_period_end)
VALUES (?,?,?,?,?,?,?,0)`, tenant, plan.Id, plan.Version, subscriptionActive,
		int64(core.TenantSubscriptionSource_TENANT_SUBSCRIPTION_SOURCE_GRANT), start, end); err != nil {
		return billingInternalError("create free tenant subscription", err)
	}
	var subscription models.TTenantSubscription
	if err := session.QueryRowCtx(ctx, &subscription, tenantSubscriptionSelect+` WHERE tenant_id=?`, tenant); err != nil {
		return billingInternalError("load free tenant subscription", err)
	}
	entitlement, err := entitlementFromPlan(tenant, 3, subscription.Id, &plan,
		subscription.CurrentPeriodStart, subscription.CurrentPeriodEnd, "")
	if err != nil {
		return err
	}
	if err := upsertEntitlementInTransaction(ctx, session, entitlement); err != nil {
		return billingInternalError("create free tenant entitlement", err)
	}
	return nil
}

func tenantBillingResponse(subscription *models.TTenantSubscription, entitlement *models.TTenantEntitlement, plan *models.TBillingPlan) *core.TenantBillingResp {
	return &core.TenantBillingResp{
		Base: okBase(), Subscription: mapTenantSubscription(subscription),
		Entitlement: mapTenantEntitlement(entitlement), Plan: mapBillingPlan(plan),
	}
}

type entitlementOverride struct {
	BuildsPerCycle      *int64 `json:"buildsPerCycle"`
	MaxBuildConcurrency *int64 `json:"maxBuildConcurrency"`
	StorageBytes        *int64 `json:"storageBytes"`
	MaxUploadBytes      *int64 `json:"maxUploadBytes"`
	TeamSeats           *int64 `json:"teamSeats"`
	ApiRateLimit        *int64 `json:"apiRateLimit"`
}

func applyEntitlementOverride(item *models.TTenantEntitlement, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		item.OverrideJson = sql.NullString{}
		return nil
	}
	var override entitlementOverride
	if err := json.Unmarshal([]byte(raw), &override); err != nil {
		return status.Error(codes.InvalidArgument, "override_json is invalid")
	}
	for _, value := range []*int64{override.BuildsPerCycle, override.MaxBuildConcurrency, override.StorageBytes, override.MaxUploadBytes, override.TeamSeats, override.ApiRateLimit} {
		if value != nil && *value < -1 {
			return status.Error(codes.InvalidArgument, "entitlement override values must be -1 or greater")
		}
	}
	if override.BuildsPerCycle != nil {
		item.BuildsPerCycle = *override.BuildsPerCycle
	}
	if override.MaxBuildConcurrency != nil {
		item.MaxBuildConcurrency = *override.MaxBuildConcurrency
	}
	if override.StorageBytes != nil {
		item.StorageBytes = *override.StorageBytes
	}
	if override.MaxUploadBytes != nil {
		item.MaxUploadBytes = *override.MaxUploadBytes
	}
	if override.TeamSeats != nil {
		item.TeamSeats = *override.TeamSeats
	}
	if override.ApiRateLimit != nil {
		item.ApiRateLimit = *override.ApiRateLimit
	}
	item.OverrideJson = sql.NullString{String: raw, Valid: true}
	return nil
}

func entitlementFromPlan(tenant, sourceType, sourceID int64, plan *models.TBillingPlan, start, end time.Time, override string) (*models.TTenantEntitlement, error) {
	item := &models.TTenantEntitlement{
		TenantId: tenant, SourceType: sourceType, SourceId: sourceID, PlanId: plan.Id, PlanVersion: plan.Version,
		BuildsPerCycle: plan.BuildsPerCycle, MaxBuildConcurrency: plan.MaxBuildConcurrency,
		StorageBytes: plan.StorageBytes, MaxUploadBytes: plan.MaxUploadBytes,
		TeamSeats: plan.TeamSeats, ApiRateLimit: plan.ApiRateLimit,
		ChargeFailedBuild: plan.ChargeFailedBuild, ChargeCacheHit: plan.ChargeCacheHit,
		ChargeRetryBuild: plan.ChargeRetryBuild, ValidFrom: start, ValidUntil: end,
		Status: entitlementActive, Revision: 1,
	}
	if err := applyEntitlementOverride(item, override); err != nil {
		return nil, err
	}
	return item, nil
}

func upsertEntitlementInTransaction(ctx context.Context, session sqlx.Session, item *models.TTenantEntitlement) error {
	var current models.TTenantEntitlement
	err := session.QueryRowCtx(ctx, &current, tenantEntitlementSelect+` WHERE tenant_id=? FOR UPDATE`, item.TenantId)
	if err == nil {
		item.Id = current.Id
		item.Revision = current.Revision + 1
		_, err = session.ExecCtx(ctx, `UPDATE t_tenant_entitlement SET source_type=?,source_id=?,plan_id=?,plan_version=?,
builds_per_cycle=?,max_build_concurrency=?,storage_bytes=?,max_upload_bytes=?,team_seats=?,api_rate_limit=?,
charge_failed_build=?,charge_cache_hit=?,charge_retry_build=?,override_json=?,valid_from=?,valid_until=?,status=?,revision=? WHERE id=?`,
			item.SourceType, item.SourceId, item.PlanId, item.PlanVersion, item.BuildsPerCycle,
			item.MaxBuildConcurrency, item.StorageBytes, item.MaxUploadBytes, item.TeamSeats, item.ApiRateLimit,
			item.ChargeFailedBuild, item.ChargeCacheHit, item.ChargeRetryBuild, item.OverrideJson,
			item.ValidFrom, item.ValidUntil, item.Status, item.Revision, item.Id)
		return err
	}
	if err != sqlx.ErrNotFound {
		return err
	}
	result, err := session.ExecCtx(ctx, `INSERT INTO t_tenant_entitlement
(tenant_id,source_type,source_id,plan_id,plan_version,builds_per_cycle,max_build_concurrency,storage_bytes,
max_upload_bytes,team_seats,api_rate_limit,charge_failed_build,charge_cache_hit,charge_retry_build,override_json,
valid_from,valid_until,status,revision) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		item.TenantId, item.SourceType, item.SourceId, item.PlanId, item.PlanVersion, item.BuildsPerCycle,
		item.MaxBuildConcurrency, item.StorageBytes, item.MaxUploadBytes, item.TeamSeats, item.ApiRateLimit,
		item.ChargeFailedBuild, item.ChargeCacheHit, item.ChargeRetryBuild, item.OverrideJson,
		item.ValidFrom, item.ValidUntil, item.Status, item.Revision)
	if err != nil {
		return err
	}
	item.Id, _ = result.LastInsertId()
	return nil
}

func loadBillingPlan(ctx context.Context, session sqlx.Session, id int64, requireActive bool) (*models.TBillingPlan, error) {
	if id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "plan_id must be greater than zero")
	}
	var item models.TBillingPlan
	if err := session.QueryRowCtx(ctx, &item, billingPlanSelect+` WHERE id=?`, id); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, status.Error(codes.NotFound, "billing plan not found")
		}
		return nil, status.Errorf(codes.Internal, "load billing plan failed: %v", err)
	}
	if requireActive && item.Status != billingPlanActive {
		return nil, status.Error(codes.FailedPrecondition, "billing plan is retired")
	}
	return &item, nil
}

func quotaExceeded(metric string, used, reserved, requested, limit int64) error {
	return status.Errorf(codes.ResourceExhausted,
		"QUOTA_EXCEEDED: metric=%s used=%d reserved=%d requested=%d limit=%d",
		metric, used, reserved, requested, limit)
}

func billingPeriodForCycle(start time.Time, cycle int64) time.Time {
	if cycle == int64(core.BillingCycle_BILLING_CYCLE_YEARLY) {
		return start.AddDate(1, 0, 0)
	}
	return start.AddDate(0, 1, 0)
}

func requireJSONOrEmpty(value, field string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var payload any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return status.Errorf(codes.InvalidArgument, "%s must be valid JSON", field)
	}
	return nil
}

func billingInternalError(operation string, err error) error {
	return status.Errorf(codes.Internal, "%s failed: %v", operation, err)
}

func billingNow() time.Time {
	return time.Now().UTC()
}

func billingInvoiceNumber(tenant int64, external string, eventID string) string {
	seed := strings.TrimSpace(external)
	if seed == "" {
		seed = strings.TrimSpace(eventID)
	}
	seed = regexp.MustCompile(`[^A-Za-z0-9_-]+`).ReplaceAllString(seed, "-")
	if len(seed) > 36 {
		seed = seed[len(seed)-36:]
	}
	return fmt.Sprintf("INV-%d-%s", tenant, seed)
}

func sessionOrDB(db sqlx.SqlConn, session sqlx.Session) sqlx.Session {
	if session != nil {
		return session
	}
	return db
}

func currentBilling(svcCtx *svc.ServiceContext, ctx context.Context, tenant int64) (*models.TTenantSubscription, *models.TTenantEntitlement, *models.TBillingPlan, error) {
	return loadTenantBilling(ctx, svcCtx.DB, tenant, false)
}
