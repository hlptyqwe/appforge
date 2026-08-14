package logic

import (
	"context"
	"database/sql"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func createBillingPlan(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CreateBillingPlanReq) (*core.BillingPlanResp, error) {
	if err := validateSystemBillingActor(ctx); err != nil {
		return nil, err
	}
	if err := validateBillingPlanInput(in); err != nil {
		return nil, err
	}
	var created models.TBillingPlan
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var latest models.TBillingPlan
		version := int64(1)
		err := session.QueryRowCtx(txCtx, &latest, billingPlanSelect+` WHERE plan_code=? ORDER BY version DESC LIMIT 1 FOR UPDATE`, in.PlanCode)
		if err == nil {
			version = latest.Version + 1
		} else if err != sqlx.ErrNotFound {
			return billingInternalError("lock latest billing plan", err)
		}
		result, err := session.ExecCtx(txCtx, `INSERT INTO t_billing_plan
(plan_code,plan_name,billing_cycle,price_amount,currency,feature_json,builds_per_cycle,max_build_concurrency,
storage_bytes,max_upload_bytes,team_seats,api_rate_limit,charge_failed_build,charge_cache_hit,
charge_retry_build,status,version) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			in.PlanCode, in.PlanName, int64(in.BillingCycle), in.PriceAmount, in.Currency, in.FeatureJson,
			in.BuildsPerCycle, in.MaxBuildConcurrency, in.StorageBytes, in.MaxUploadBytes,
			in.TeamSeats, in.ApiRateLimit, boolInt(in.ChargeFailedBuild), boolInt(in.ChargeCacheHit),
			boolInt(in.ChargeRetryBuild), billingPlanActive, version)
		if err != nil {
			return status.Errorf(codes.Aborted, "create billing plan version failed; retry request: %v", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return billingInternalError("read billing plan id", err)
		}
		if err := session.QueryRowCtx(txCtx, &created, billingPlanSelect+` WHERE id=?`, id); err != nil {
			return billingInternalError("load created billing plan", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &core.BillingPlanResp{Base: okBase(), Data: mapBillingPlan(&created)}, nil
}

func getBillingPlan(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BillingPlanIdReq) (*core.BillingPlanResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	plan, err := loadBillingPlan(ctx, svcCtx.DB, in.Id, false)
	if err != nil {
		return nil, err
	}
	return &core.BillingPlanResp{Base: okBase(), Data: mapBillingPlan(plan)}, nil
}

func listBillingPlans(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BillingPlanListReq) (*core.BillingPlanListResp, error) {
	if in == nil {
		in = &core.BillingPlanListReq{}
	}
	limit, offset := normalizeBillingPage(in.Page, in.PageSize)
	where := "1=1"
	args := make([]any, 0, 3)
	if in.Status != core.BillingPlanStatus_BILLING_PLAN_STATUS_UNKNOWN {
		if in.Status != core.BillingPlanStatus_BILLING_PLAN_STATUS_ACTIVE && in.Status != core.BillingPlanStatus_BILLING_PLAN_STATUS_RETIRED {
			return nil, status.Error(codes.InvalidArgument, "status is invalid")
		}
		where = "status=?"
		args = append(args, int64(in.Status))
	}
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, `SELECT COUNT(*) FROM t_billing_plan WHERE `+where, args...); err != nil {
		return nil, billingInternalError("count billing plans", err)
	}
	queryArgs := append(append([]any{}, args...), limit, offset)
	var rows []models.TBillingPlan
	if err := svcCtx.DB.QueryRowsCtx(ctx, &rows, billingPlanSelect+` WHERE `+where+` ORDER BY plan_code ASC,version DESC LIMIT ? OFFSET ?`, queryArgs...); err != nil {
		return nil, billingInternalError("list billing plans", err)
	}
	items := make([]*core.BillingPlan, 0, len(rows))
	for i := range rows {
		items = append(items, mapBillingPlan(&rows[i]))
	}
	return &core.BillingPlanListResp{Base: okBase(), Data: items, Total: total}, nil
}

func retireBillingPlan(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BillingPlanIdReq) (*core.BillingPlanResp, error) {
	if err := validateSystemBillingActor(ctx); err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	if _, err := svcCtx.DB.ExecCtx(ctx, `UPDATE t_billing_plan SET status=? WHERE id=? AND status=?`, billingPlanRetired, in.Id, billingPlanActive); err != nil {
		return nil, billingInternalError("retire billing plan", err)
	}
	return getBillingPlan(ctx, svcCtx, in)
}

func getTenantBilling(ctx context.Context, svcCtx *svc.ServiceContext, in *core.TenantBillingReq) (*core.TenantBillingResp, error) {
	if in == nil {
		in = &core.TenantBillingReq{}
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	subscription, entitlement, plan, err := currentBilling(svcCtx, ctx, tenant)
	if err != nil {
		return nil, err
	}
	return tenantBillingResponse(subscription, entitlement, plan), nil
}

func upsertManualSubscription(ctx context.Context, svcCtx *svc.ServiceContext, in *core.UpsertManualSubscriptionReq) (*core.TenantBillingResp, error) {
	if err := validateSystemBillingActor(ctx); err != nil {
		return nil, err
	}
	if in == nil || in.TenantId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	start, end := timeFromMillis(in.PeriodStart), timeFromMillis(in.PeriodEnd)
	if in.PeriodStart <= 0 || in.PeriodEnd <= in.PeriodStart || !end.After(start) {
		return nil, status.Error(codes.InvalidArgument, "billing period is invalid")
	}
	if err := requireOptionalText(in.ContractReference, "contract_reference", 255); err != nil {
		return nil, err
	}
	if err := requireJSONOrEmpty(in.OverrideJson, "override_json"); err != nil {
		return nil, err
	}
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		plan, err := loadBillingPlan(txCtx, session, in.PlanId, true)
		if err != nil {
			return err
		}
		var subscription models.TTenantSubscription
		err = session.QueryRowCtx(txCtx, &subscription, tenantSubscriptionSelect+` WHERE tenant_id=? FOR UPDATE`, in.TenantId)
		grace := sql.NullTime{}
		if in.GraceUntil > 0 {
			grace = sql.NullTime{Time: timeFromMillis(in.GraceUntil), Valid: true}
		}
		contract := nullString(in.ContractReference)
		if err == sqlx.ErrNotFound {
			result, insertErr := session.ExecCtx(txCtx, `INSERT INTO t_tenant_subscription
(tenant_id,plan_id,plan_version,status,source,external_customer_id,external_subscription_id,current_period_start,
current_period_end,cancel_at_period_end,grace_until,pending_plan_id,pending_plan_version)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, in.TenantId, plan.Id, plan.Version, subscriptionActive,
				int64(core.TenantSubscriptionSource_TENANT_SUBSCRIPTION_SOURCE_MANUAL), sql.NullString{}, contract,
				start, end, 0, grace, 0, 0)
			if insertErr != nil {
				return billingInternalError("create manual subscription", insertErr)
			}
			subscription.Id, _ = result.LastInsertId()
		} else if err != nil {
			return billingInternalError("lock tenant subscription", err)
		} else {
			subscription.PlanId = plan.Id
			subscription.PlanVersion = plan.Version
			subscription.Status = subscriptionActive
			subscription.Source = int64(core.TenantSubscriptionSource_TENANT_SUBSCRIPTION_SOURCE_MANUAL)
			subscription.ExternalCustomerId = sql.NullString{}
			subscription.ExternalSubscriptionId = contract
			subscription.CurrentPeriodStart = start
			subscription.CurrentPeriodEnd = end
			subscription.CancelAtPeriodEnd = 0
			subscription.GraceUntil = grace
			subscription.PendingPlanId = 0
			subscription.PendingPlanVersion = 0
			if _, updateErr := session.ExecCtx(txCtx, `UPDATE t_tenant_subscription SET plan_id=?,plan_version=?,status=?,source=?,
external_customer_id=NULL,external_subscription_id=?,current_period_start=?,current_period_end=?,cancel_at_period_end=0,
grace_until=?,pending_plan_id=0,pending_plan_version=0 WHERE id=?`, plan.Id, plan.Version,
				subscription.Status, subscription.Source, contract, start, end, grace, subscription.Id); updateErr != nil {
				return billingInternalError("update manual subscription", updateErr)
			}
		}
		entitlement, err := entitlementFromPlan(in.TenantId, 2, subscription.Id, plan, start, end, in.OverrideJson)
		if err != nil {
			return err
		}
		if err := upsertEntitlementInTransaction(txCtx, session, entitlement); err != nil {
			return billingInternalError("refresh manual entitlement", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return loadTenantBillingResponse(ctx, svcCtx, in.TenantId)
}

func changeTenantSubscription(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ChangeTenantSubscriptionReq) (*core.TenantBillingResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	if in.Mode != core.SubscriptionChangeMode_SUBSCRIPTION_CHANGE_MODE_IMMEDIATE && in.Mode != core.SubscriptionChangeMode_SUBSCRIPTION_CHANGE_MODE_PERIOD_END {
		return nil, status.Error(codes.InvalidArgument, "subscription change mode is invalid")
	}
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		subscription, entitlement, _, err := loadTenantBilling(txCtx, session, tenant, true)
		if err != nil {
			return err
		}
		plan, err := loadBillingPlan(txCtx, session, in.PlanId, true)
		if err != nil {
			return err
		}
		if in.Mode == core.SubscriptionChangeMode_SUBSCRIPTION_CHANGE_MODE_PERIOD_END {
			_, err = session.ExecCtx(txCtx, `UPDATE t_tenant_subscription SET pending_plan_id=?,pending_plan_version=? WHERE id=?`, plan.Id, plan.Version, subscription.Id)
			return err
		}
		if _, err = session.ExecCtx(txCtx, `UPDATE t_tenant_subscription SET plan_id=?,plan_version=?,status=?,
pending_plan_id=0,pending_plan_version=0,cancel_at_period_end=0 WHERE id=?`, plan.Id, plan.Version, subscriptionActive, subscription.Id); err != nil {
			return billingInternalError("change subscription", err)
		}
		sourceType := subscription.Source
		if sourceType < 1 || sourceType > 3 {
			sourceType = 1
		}
		updated, err := entitlementFromPlan(tenant, sourceType, subscription.Id, plan,
			subscription.CurrentPeriodStart, subscription.CurrentPeriodEnd, stringValue(entitlement.OverrideJson))
		if err != nil {
			return err
		}
		if err := upsertEntitlementInTransaction(txCtx, session, updated); err != nil {
			return billingInternalError("refresh changed entitlement", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return loadTenantBillingResponse(ctx, svcCtx, tenant)
}

func cancelTenantSubscription(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CancelTenantSubscriptionReq) (*core.TenantBillingResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		subscription, _, _, err := loadTenantBilling(txCtx, session, tenant, true)
		if err != nil {
			return err
		}
		if in.Immediately {
			if _, err := session.ExecCtx(txCtx, `UPDATE t_tenant_subscription SET status=?,cancel_at_period_end=0,
pending_plan_id=0,pending_plan_version=0 WHERE id=?`, subscriptionCanceled, subscription.Id); err != nil {
				return billingInternalError("cancel subscription", err)
			}
			if _, err := session.ExecCtx(txCtx, `UPDATE t_tenant_entitlement SET status=?,revision=revision+1 WHERE tenant_id=?`, entitlementPaused, tenant); err != nil {
				return billingInternalError("pause canceled entitlement", err)
			}
			return nil
		}
		_, err = session.ExecCtx(txCtx, `UPDATE t_tenant_subscription SET cancel_at_period_end=1 WHERE id=?`, subscription.Id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return loadTenantBillingResponse(ctx, svcCtx, tenant)
}

func loadTenantBillingResponse(ctx context.Context, svcCtx *svc.ServiceContext, tenant int64) (*core.TenantBillingResp, error) {
	subscription, entitlement, plan, err := currentBilling(svcCtx, ctx, tenant)
	if err != nil {
		return nil, err
	}
	return tenantBillingResponse(subscription, entitlement, plan), nil
}

func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func normalizeContractReference(value string) sql.NullString {
	return nullString(strings.TrimSpace(value))
}
