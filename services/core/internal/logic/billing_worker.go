package logic

import (
	"context"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type BillingWorker struct {
	svcCtx       *svc.ServiceContext
	pollInterval time.Duration
}

func NewBillingWorker(svcCtx *svc.ServiceContext) *BillingWorker {
	interval := svcCtx.Config.BillingWorker.PollInterval
	if interval <= 0 {
		interval = time.Minute
	}
	return &BillingWorker{svcCtx: svcCtx, pollInterval: interval}
}

func (w *BillingWorker) Start(ctx context.Context) {
	if w == nil || w.svcCtx == nil || !w.svcCtx.Config.BillingWorker.Enabled {
		return
	}
	go func() {
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()
		w.runCycle(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.runCycle(ctx)
			}
		}
	}()
}

func (w *BillingWorker) runCycle(ctx context.Context) {
	if _, err := w.svcCtx.DB.ExecCtx(ctx, `UPDATE t_quota_reservation SET status=?,released_at=CURRENT_TIMESTAMP(3)
WHERE status=? AND expires_at<=CURRENT_TIMESTAMP(3)`, quotaExpired, quotaReserved); err != nil {
		logx.WithContext(ctx).Errorf("expire billing quota reservations failed: %v", err)
	}
	if _, err := w.svcCtx.DB.ExecCtx(ctx, `UPDATE t_billing_webhook_event SET payload_ciphertext='expired'
WHERE retain_until<CURRENT_TIMESTAMP(3) AND payload_ciphertext<>'expired'`); err != nil {
		logx.WithContext(ctx).Errorf("expire billing webhook payloads failed: %v", err)
	}
	var ids []int64
	if err := w.svcCtx.DB.QueryRowsCtx(ctx, &ids, `SELECT id FROM t_tenant_subscription
WHERE (status IN (?,?) AND (current_period_end<=CURRENT_TIMESTAMP(3) OR (grace_until IS NOT NULL AND grace_until<=CURRENT_TIMESTAMP(3))))
ORDER BY id LIMIT 100`, subscriptionActive, subscriptionGrace); err != nil {
		logx.WithContext(ctx).Errorf("list due subscriptions failed: %v", err)
		return
	}
	for _, id := range ids {
		if err := w.processSubscription(ctx, id); err != nil {
			logx.WithContext(ctx).Errorf("process due subscription failed: subscriptionId=%d err=%v", id, err)
		}
	}
}

func (w *BillingWorker) processSubscription(ctx context.Context, id int64) error {
	return w.svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var subscription models.TTenantSubscription
		if err := session.QueryRowCtx(txCtx, &subscription, tenantSubscriptionSelect+` WHERE id=? FOR UPDATE`, id); err != nil {
			return err
		}
		now := billingNow()
		if subscription.Status == subscriptionGrace {
			if subscription.GraceUntil.Valid && !now.Before(subscription.GraceUntil.Time) {
				if _, err := session.ExecCtx(txCtx, `UPDATE t_tenant_subscription SET status=? WHERE id=?`, subscriptionPaused, id); err != nil {
					return err
				}
				_, err := session.ExecCtx(txCtx, `UPDATE t_tenant_entitlement SET status=?,revision=revision+1 WHERE tenant_id=?`, entitlementPaused, subscription.TenantId)
				return err
			}
			return nil
		}
		if subscription.Status != subscriptionActive || now.Before(subscription.CurrentPeriodEnd) {
			return nil
		}
		if subscription.CancelAtPeriodEnd == 1 {
			if _, err := session.ExecCtx(txCtx, `UPDATE t_tenant_subscription SET status=? WHERE id=?`, subscriptionCanceled, id); err != nil {
				return err
			}
			_, err := session.ExecCtx(txCtx, `UPDATE t_tenant_entitlement SET status=?,revision=revision+1 WHERE tenant_id=?`, entitlementPaused, subscription.TenantId)
			return err
		}
		if subscription.Source == int64(core.TenantSubscriptionSource_TENANT_SUBSCRIPTION_SOURCE_STRIPE) {
			grace := now.Add(72 * time.Hour)
			if _, err := session.ExecCtx(txCtx, `UPDATE t_tenant_subscription SET status=?,grace_until=? WHERE id=?`, subscriptionGrace, grace, id); err != nil {
				return err
			}
			_, err := session.ExecCtx(txCtx, `UPDATE t_tenant_entitlement SET valid_until=?,status=?,revision=revision+1 WHERE tenant_id=?`, grace, entitlementActive, subscription.TenantId)
			return err
		}
		if subscription.Source == int64(core.TenantSubscriptionSource_TENANT_SUBSCRIPTION_SOURCE_MANUAL) {
			if subscription.GraceUntil.Valid && now.Before(subscription.GraceUntil.Time) {
				if _, err := session.ExecCtx(txCtx, `UPDATE t_tenant_subscription SET status=? WHERE id=?`, subscriptionGrace, id); err != nil {
					return err
				}
				_, err := session.ExecCtx(txCtx, `UPDATE t_tenant_entitlement SET valid_until=?,revision=revision+1 WHERE tenant_id=?`, subscription.GraceUntil.Time, subscription.TenantId)
				return err
			}
			if _, err := session.ExecCtx(txCtx, `UPDATE t_tenant_subscription SET status=? WHERE id=?`, subscriptionPaused, id); err != nil {
				return err
			}
			_, err := session.ExecCtx(txCtx, `UPDATE t_tenant_entitlement SET status=?,revision=revision+1 WHERE tenant_id=?`, entitlementPaused, subscription.TenantId)
			return err
		}
		planID := subscription.PlanId
		if subscription.PendingPlanId > 0 {
			planID = subscription.PendingPlanId
		}
		plan, err := loadBillingPlan(txCtx, session, planID, false)
		if err != nil {
			return err
		}
		start := subscription.CurrentPeriodEnd
		end := billingPeriodForCycle(start, plan.BillingCycle)
		if _, err := session.ExecCtx(txCtx, `UPDATE t_tenant_subscription SET plan_id=?,plan_version=?,status=?,
current_period_start=?,current_period_end=?,pending_plan_id=0,pending_plan_version=0 WHERE id=?`,
			plan.Id, plan.Version, subscriptionActive, start, end, id); err != nil {
			return err
		}
		var current models.TTenantEntitlement
		if err := session.QueryRowCtx(txCtx, &current, tenantEntitlementSelect+` WHERE tenant_id=? FOR UPDATE`, subscription.TenantId); err != nil {
			return err
		}
		entitlement, err := entitlementFromPlan(subscription.TenantId, 3, id, plan, start, end, stringValue(current.OverrideJson))
		if err != nil {
			return err
		}
		return upsertEntitlementInTransaction(txCtx, session, entitlement)
	})
}
