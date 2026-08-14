package logic

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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

func listInvoices(ctx context.Context, svcCtx *svc.ServiceContext, in *core.InvoiceListReq) (*core.InvoiceListResp, error) {
	if in == nil {
		in = &core.InvoiceListReq{}
	}
	tenant, err := billingTargetTenant(ctx, in.TenantId)
	if err != nil {
		return nil, err
	}
	limit, offset := normalizeBillingPage(in.Page, in.PageSize)
	where := "tenant_id=?"
	args := []any{tenant}
	if in.Status != core.InvoiceStatus_INVOICE_STATUS_UNKNOWN {
		if in.Status < core.InvoiceStatus_INVOICE_STATUS_DRAFT || in.Status > core.InvoiceStatus_INVOICE_STATUS_REFUNDED {
			return nil, status.Error(codes.InvalidArgument, "invoice status is invalid")
		}
		where += " AND status=?"
		args = append(args, int64(in.Status))
	}
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, `SELECT COUNT(*) FROM t_invoice WHERE `+where, args...); err != nil {
		return nil, billingInternalError("count invoices", err)
	}
	queryArgs := append(append([]any{}, args...), limit, offset)
	var rows []models.TInvoice
	if err := svcCtx.DB.QueryRowsCtx(ctx, &rows, invoiceSelect+` WHERE `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, queryArgs...); err != nil {
		return nil, billingInternalError("list invoices", err)
	}
	items := make([]*core.Invoice, 0, len(rows))
	for i := range rows {
		invoice, err := mapInvoiceWithItems(ctx, svcCtx.DB, &rows[i])
		if err != nil {
			return nil, err
		}
		items = append(items, invoice)
	}
	return &core.InvoiceListResp{Base: okBase(), Data: items, Total: total}, nil
}

func mapInvoiceWithItems(ctx context.Context, session sqlx.Session, item *models.TInvoice) (*core.Invoice, error) {
	if item == nil {
		return nil, nil
	}
	var rows []models.TInvoiceItem
	if err := session.QueryRowsCtx(ctx, &rows, invoiceItemSelect+` WHERE tenant_id=? AND invoice_id=? ORDER BY id ASC`, item.TenantId, item.Id); err != nil {
		return nil, billingInternalError("list invoice items", err)
	}
	items := make([]*core.InvoiceItem, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		items = append(items, &core.InvoiceItem{Id: row.Id, InvoiceId: row.InvoiceId, LineKey: row.LineKey,
			ItemType: int32(row.ItemType), Description: row.Description, Metric: parseUsageMetric(stringValue(row.Metric)),
			Quantity: row.Quantity, UnitAmount: row.UnitAmount, Amount: row.Amount})
	}
	return &core.Invoice{
		Id: item.Id, TenantId: item.TenantId, InvoiceNo: item.InvoiceNo,
		ExternalInvoiceId: stringValue(item.ExternalInvoiceId), Status: core.InvoiceStatus(item.Status),
		Currency: item.Currency, SubtotalAmount: item.SubtotalAmount, DiscountAmount: item.DiscountAmount,
		TaxAmount: item.TaxAmount, TotalAmount: item.TotalAmount, PaidAmount: item.PaidAmount,
		RefundedAmount: item.RefundedAmount, PeriodStart: millis(item.PeriodStart), PeriodEnd: millis(item.PeriodEnd),
		DueAt: timeValue(item.DueAt), PaidAt: timeValue(item.PaidAt), Items: items, CreateTime: millis(item.CreateTime),
	}, nil
}

func applyBillingWebhook(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ApplyBillingWebhookReq) (*core.RespBase, error) {
	if err := validateBillingWebhook(in); err != nil {
		return nil, err
	}
	payloadHash := sha256.Sum256([]byte(in.PayloadJson))
	payloadSHA := hex.EncodeToString(payloadHash[:])
	ciphertext, err := svcCtx.Secrets.Seal(in.PayloadJson)
	if err != nil {
		return nil, billingInternalError("encrypt billing webhook payload", err)
	}
	eventTime := time.Unix(in.EventCreatedAt, 0).UTC()
	result, err := svcCtx.DB.ExecCtx(ctx, `INSERT INTO t_billing_webhook_event
(provider,provider_event_id,event_type,event_created_at,payload_sha256,payload_ciphertext,status,attempt,tenant_id,retain_until)
VALUES (?,?,?,?,?,?,1,0,?,?) ON DUPLICATE KEY UPDATE id=LAST_INSERT_ID(id)`, strings.ToLower(strings.TrimSpace(in.Provider)),
		strings.TrimSpace(in.ProviderEventId), strings.TrimSpace(in.EventType), eventTime, payloadSHA, ciphertext,
		in.TenantId, billingNow().AddDate(0, 3, 0))
	if err != nil {
		return nil, billingInternalError("store billing webhook event", err)
	}
	eventID, err := result.LastInsertId()
	if err != nil {
		return nil, billingInternalError("read billing webhook event id", err)
	}
	var event models.TBillingWebhookEvent
	if err := svcCtx.DB.QueryRowCtx(ctx, &event, `SELECT id,provider,provider_event_id,event_type,event_created_at,
payload_sha256,payload_ciphertext,status,attempt,tenant_id,error_message,processed_at,retain_until,create_time,update_time
FROM t_billing_webhook_event WHERE id=?`, eventID); err != nil {
		return nil, billingInternalError("load billing webhook event", err)
	}
	if event.PayloadSha256 != payloadSHA || event.EventType != strings.TrimSpace(in.EventType) {
		return nil, status.Error(codes.AlreadyExists, "billing provider event id was reused with different payload")
	}
	if event.Status == 2 || event.Status == 3 {
		return &core.RespBase{Base: okBase()}, nil
	}
	applyErr := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var locked models.TBillingWebhookEvent
		if err := session.QueryRowCtx(txCtx, &locked, `SELECT id,provider,provider_event_id,event_type,event_created_at,
payload_sha256,payload_ciphertext,status,attempt,tenant_id,error_message,processed_at,retain_until,create_time,update_time
FROM t_billing_webhook_event WHERE id=? FOR UPDATE`, eventID); err != nil {
			return billingInternalError("lock billing webhook event", err)
		}
		if locked.Status == 2 || locked.Status == 3 {
			return nil
		}
		applied, tenant, err := applyNormalizedBillingEvent(txCtx, session, in, eventTime)
		if err != nil {
			return err
		}
		finalStatus := int64(3)
		if applied {
			finalStatus = 2
		}
		_, err = session.ExecCtx(txCtx, `UPDATE t_billing_webhook_event SET status=?,attempt=attempt+1,
tenant_id=?,error_message=NULL,processed_at=CURRENT_TIMESTAMP(3) WHERE id=?`, finalStatus, tenant, eventID)
		return err
	})
	if applyErr != nil {
		_, _ = svcCtx.DB.ExecCtx(ctx, `UPDATE t_billing_webhook_event SET status=4,attempt=attempt+1,
error_message=?,processed_at=CURRENT_TIMESTAMP(3) WHERE id=?`, truncateBillingError(applyErr.Error()), eventID)
		return nil, applyErr
	}
	return &core.RespBase{Base: okBase()}, nil
}

func validateBillingWebhook(in *core.ApplyBillingWebhookReq) error {
	if in == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if strings.ToLower(strings.TrimSpace(in.Provider)) != "stripe" {
		return status.Error(codes.InvalidArgument, "billing provider is not supported")
	}
	if err := requireText(in.ProviderEventId, "provider_event_id", 255); err != nil {
		return err
	}
	if err := requireText(in.EventType, "event_type", 128); err != nil {
		return err
	}
	if in.EventCreatedAt <= 0 {
		return status.Error(codes.InvalidArgument, "event_created_at is required")
	}
	if err := requireJSONOrEmpty(in.PayloadJson, "payload_json"); err != nil || strings.TrimSpace(in.PayloadJson) == "" {
		if err != nil {
			return err
		}
		return status.Error(codes.InvalidArgument, "payload_json is required")
	}
	return nil
}

func applyNormalizedBillingEvent(ctx context.Context, session sqlx.Session, in *core.ApplyBillingWebhookReq, eventTime time.Time) (bool, int64, error) {
	eventType := strings.TrimSpace(in.EventType)
	switch eventType {
	case "checkout.session.completed", "invoice.paid", "customer.subscription.updated":
		subscription, plan, err := activateStripeSubscription(ctx, session, in, eventTime)
		if err != nil {
			return false, in.TenantId, err
		}
		if subscription == nil {
			return false, in.TenantId, nil
		}
		if eventType == "invoice.paid" || eventType == "checkout.session.completed" {
			invoice, err := upsertSignedInvoice(ctx, session, subscription, plan, in, core.InvoiceStatus_INVOICE_STATUS_PAID)
			if err != nil {
				return false, subscription.TenantId, err
			}
			if strings.TrimSpace(in.ExternalTransactionId) != "" {
				if err := insertPaymentTransaction(ctx, session, subscription.TenantId, invoiceID(invoice), in,
					core.PaymentTransactionType_PAYMENT_TRANSACTION_TYPE_CHARGE,
					core.PaymentTransactionStatus_PAYMENT_TRANSACTION_STATUS_SUCCEEDED, eventTime); err != nil {
					return false, subscription.TenantId, err
				}
			}
		}
		return true, subscription.TenantId, nil
	case "invoice.payment_failed":
		subscription, err := findBillingEventSubscription(ctx, session, in)
		if err != nil {
			return false, in.TenantId, err
		}
		if subscription == nil {
			return false, in.TenantId, nil
		}
		if isStaleSubscriptionEvent(subscription, eventTime) {
			return false, subscription.TenantId, nil
		}
		grace := billingNow().Add(72 * time.Hour)
		if _, err := session.ExecCtx(ctx, `UPDATE t_tenant_subscription SET status=?,grace_until=?,last_provider_event_at=? WHERE id=?`,
			subscriptionGrace, grace, eventTime, subscription.Id); err != nil {
			return false, subscription.TenantId, billingInternalError("mark subscription payment failed", err)
		}
		if _, err := session.ExecCtx(ctx, `UPDATE t_tenant_entitlement SET status=?,valid_until=GREATEST(valid_until,?),revision=revision+1 WHERE tenant_id=?`,
			entitlementActive, grace, subscription.TenantId); err != nil {
			return false, subscription.TenantId, billingInternalError("extend grace entitlement", err)
		}
		plan, err := loadBillingPlan(ctx, session, subscription.PlanId, false)
		if err != nil {
			return false, subscription.TenantId, err
		}
		invoice, err := upsertSignedInvoice(ctx, session, subscription, plan, in, core.InvoiceStatus_INVOICE_STATUS_PAYMENT_FAILED)
		if err != nil {
			return false, subscription.TenantId, err
		}
		if strings.TrimSpace(in.ExternalTransactionId) != "" {
			if err := insertPaymentTransaction(ctx, session, subscription.TenantId, invoiceID(invoice), in,
				core.PaymentTransactionType_PAYMENT_TRANSACTION_TYPE_CHARGE,
				core.PaymentTransactionStatus_PAYMENT_TRANSACTION_STATUS_FAILED, eventTime); err != nil {
				return false, subscription.TenantId, err
			}
		}
		return true, subscription.TenantId, nil
	case "customer.subscription.deleted":
		subscription, err := findBillingEventSubscription(ctx, session, in)
		if err != nil {
			return false, in.TenantId, err
		}
		if subscription == nil || isStaleSubscriptionEvent(subscription, eventTime) {
			return false, in.TenantId, nil
		}
		if _, err := session.ExecCtx(ctx, `UPDATE t_tenant_subscription SET status=?,cancel_at_period_end=0,
last_provider_event_at=? WHERE id=?`, subscriptionCanceled, eventTime, subscription.Id); err != nil {
			return false, subscription.TenantId, billingInternalError("cancel stripe subscription", err)
		}
		if _, err := session.ExecCtx(ctx, `UPDATE t_tenant_entitlement SET status=?,revision=revision+1 WHERE tenant_id=?`, entitlementPaused, subscription.TenantId); err != nil {
			return false, subscription.TenantId, billingInternalError("pause canceled stripe entitlement", err)
		}
		return true, subscription.TenantId, nil
	case "charge.refunded":
		invoice, err := findInvoiceByExternalID(ctx, session, in.ExternalInvoiceId)
		if err != nil {
			return false, in.TenantId, err
		}
		if invoice == nil {
			return false, in.TenantId, nil
		}
		newRefunded := invoice.RefundedAmount + in.Amount
		if newRefunded > invoice.PaidAmount {
			newRefunded = invoice.PaidAmount
		}
		invoiceStatus := invoice.Status
		if newRefunded >= invoice.TotalAmount {
			invoiceStatus = int64(core.InvoiceStatus_INVOICE_STATUS_REFUNDED)
		}
		if _, err := session.ExecCtx(ctx, `UPDATE t_invoice SET refunded_amount=?,status=? WHERE id=?`, newRefunded, invoiceStatus, invoice.Id); err != nil {
			return false, invoice.TenantId, billingInternalError("apply invoice refund", err)
		}
		if err := insertPaymentTransaction(ctx, session, invoice.TenantId, invoice.Id, in,
			core.PaymentTransactionType_PAYMENT_TRANSACTION_TYPE_REFUND,
			core.PaymentTransactionStatus_PAYMENT_TRANSACTION_STATUS_SUCCEEDED, eventTime); err != nil {
			return false, invoice.TenantId, err
		}
		return true, invoice.TenantId, nil
	case "charge.dispute.created":
		invoice, err := findInvoiceByExternalID(ctx, session, in.ExternalInvoiceId)
		if err != nil {
			return false, in.TenantId, err
		}
		tenant := in.TenantId
		invoiceRef := int64(0)
		if invoice != nil {
			tenant, invoiceRef = invoice.TenantId, invoice.Id
		}
		if tenant <= 0 {
			return false, 0, nil
		}
		if err := insertPaymentTransaction(ctx, session, tenant, invoiceRef, in,
			core.PaymentTransactionType_PAYMENT_TRANSACTION_TYPE_DISPUTE,
			core.PaymentTransactionStatus_PAYMENT_TRANSACTION_STATUS_PENDING, eventTime); err != nil {
			return false, tenant, err
		}
		return true, tenant, nil
	default:
		return false, in.TenantId, nil
	}
}

func activateStripeSubscription(ctx context.Context, session sqlx.Session, in *core.ApplyBillingWebhookReq, eventTime time.Time) (*models.TTenantSubscription, *models.TBillingPlan, error) {
	subscription, err := findBillingEventSubscription(ctx, session, in)
	if err != nil {
		return nil, nil, err
	}
	tenant := in.TenantId
	planID := in.PlanId
	if subscription != nil {
		tenant = subscription.TenantId
		if planID <= 0 {
			planID = subscription.PlanId
		}
		if isStaleSubscriptionEvent(subscription, eventTime) {
			plan, _ := loadBillingPlan(ctx, session, subscription.PlanId, false)
			return nil, plan, nil
		}
	}
	if tenant <= 0 || planID <= 0 {
		return nil, nil, status.Error(codes.FailedPrecondition, "signed billing event is missing tenant or plan metadata")
	}
	plan, err := loadBillingPlan(ctx, session, planID, false)
	if err != nil {
		return nil, nil, err
	}
	periodStart := billingNow()
	if in.PeriodStart > 0 {
		periodStart = time.Unix(in.PeriodStart, 0).UTC()
	}
	periodEnd := billingPeriodForCycle(periodStart, plan.BillingCycle)
	if in.PeriodEnd > in.PeriodStart {
		periodEnd = time.Unix(in.PeriodEnd, 0).UTC()
	}
	if subscription == nil {
		result, err := session.ExecCtx(ctx, `INSERT INTO t_tenant_subscription
(tenant_id,plan_id,plan_version,status,source,external_customer_id,external_subscription_id,current_period_start,
current_period_end,cancel_at_period_end,last_provider_event_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, tenant,
			plan.Id, plan.Version, subscriptionActive, int64(core.TenantSubscriptionSource_TENANT_SUBSCRIPTION_SOURCE_STRIPE),
			nullString(in.ExternalCustomerId), nullString(in.ExternalSubscriptionId), periodStart, periodEnd,
			boolInt(in.CancelAtPeriodEnd), eventTime)
		if err != nil {
			return nil, nil, billingInternalError("create stripe subscription", err)
		}
		id, _ := result.LastInsertId()
		subscription = &models.TTenantSubscription{Id: id, TenantId: tenant, PlanId: plan.Id, PlanVersion: plan.Version,
			Status: subscriptionActive, Source: int64(core.TenantSubscriptionSource_TENANT_SUBSCRIPTION_SOURCE_STRIPE),
			ExternalCustomerId: nullString(in.ExternalCustomerId), ExternalSubscriptionId: nullString(in.ExternalSubscriptionId),
			CurrentPeriodStart: periodStart, CurrentPeriodEnd: periodEnd, CancelAtPeriodEnd: boolInt(in.CancelAtPeriodEnd),
			LastProviderEventAt: sql.NullTime{Time: eventTime, Valid: true}}
	} else {
		if _, err := session.ExecCtx(ctx, `UPDATE t_tenant_subscription SET plan_id=?,plan_version=?,status=?,source=?,
external_customer_id=?,external_subscription_id=?,current_period_start=?,current_period_end=?,cancel_at_period_end=?,
grace_until=NULL,last_provider_event_at=? WHERE id=?`, plan.Id, plan.Version, subscriptionActive,
			int64(core.TenantSubscriptionSource_TENANT_SUBSCRIPTION_SOURCE_STRIPE), nullString(in.ExternalCustomerId),
			nullString(in.ExternalSubscriptionId), periodStart, periodEnd, boolInt(in.CancelAtPeriodEnd), eventTime, subscription.Id); err != nil {
			return nil, nil, billingInternalError("update stripe subscription", err)
		}
		subscription.PlanId, subscription.PlanVersion = plan.Id, plan.Version
		subscription.Status = subscriptionActive
		subscription.Source = int64(core.TenantSubscriptionSource_TENANT_SUBSCRIPTION_SOURCE_STRIPE)
		subscription.ExternalCustomerId = nullString(in.ExternalCustomerId)
		subscription.ExternalSubscriptionId = nullString(in.ExternalSubscriptionId)
		subscription.CurrentPeriodStart, subscription.CurrentPeriodEnd = periodStart, periodEnd
		subscription.CancelAtPeriodEnd = boolInt(in.CancelAtPeriodEnd)
		subscription.LastProviderEventAt = sql.NullTime{Time: eventTime, Valid: true}
	}
	entitlement, err := entitlementFromPlan(tenant, 1, subscription.Id, plan, periodStart, periodEnd, "")
	if err != nil {
		return nil, nil, err
	}
	if err := upsertEntitlementInTransaction(ctx, session, entitlement); err != nil {
		return nil, nil, billingInternalError("refresh stripe entitlement", err)
	}
	return subscription, plan, nil
}

func findBillingEventSubscription(ctx context.Context, session sqlx.Session, in *core.ApplyBillingWebhookReq) (*models.TTenantSubscription, error) {
	var item models.TTenantSubscription
	if strings.TrimSpace(in.ExternalSubscriptionId) != "" {
		err := session.QueryRowCtx(ctx, &item, tenantSubscriptionSelect+` WHERE source=? AND external_subscription_id=? FOR UPDATE`,
			int64(core.TenantSubscriptionSource_TENANT_SUBSCRIPTION_SOURCE_STRIPE), strings.TrimSpace(in.ExternalSubscriptionId))
		if err == nil {
			return &item, nil
		}
		if err != sqlx.ErrNotFound {
			return nil, billingInternalError("find billing event subscription", err)
		}
	}
	if in.TenantId <= 0 {
		return nil, nil
	}
	err := session.QueryRowCtx(ctx, &item, tenantSubscriptionSelect+` WHERE tenant_id=? FOR UPDATE`, in.TenantId)
	if err == sqlx.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, billingInternalError("find billing event subscription", err)
	}
	return &item, nil
}

func isStaleSubscriptionEvent(item *models.TTenantSubscription, eventTime time.Time) bool {
	return item != nil && item.LastProviderEventAt.Valid && eventTime.Before(item.LastProviderEventAt.Time)
}

func upsertSignedInvoice(ctx context.Context, session sqlx.Session, subscription *models.TTenantSubscription,
	plan *models.TBillingPlan, in *core.ApplyBillingWebhookReq, targetStatus core.InvoiceStatus,
) (*models.TInvoice, error) {
	if strings.TrimSpace(in.ExternalInvoiceId) == "" && strings.TrimSpace(in.ExternalTransactionId) == "" {
		return nil, nil
	}
	externalID := strings.TrimSpace(in.ExternalInvoiceId)
	if externalID == "" {
		externalID = "checkout-" + strings.TrimSpace(in.ExternalTransactionId)
	}
	amount := in.Amount
	if amount <= 0 {
		amount = plan.PriceAmount
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = plan.Currency
	}
	if !billingCurrencyPattern.MatchString(currency) || amount < 0 {
		return nil, status.Error(codes.InvalidArgument, "signed invoice amount or currency is invalid")
	}
	periodStart, periodEnd := subscription.CurrentPeriodStart, subscription.CurrentPeriodEnd
	if in.PeriodStart > 0 {
		periodStart = time.Unix(in.PeriodStart, 0).UTC()
	}
	if in.PeriodEnd > in.PeriodStart {
		periodEnd = time.Unix(in.PeriodEnd, 0).UTC()
	}
	var existing models.TInvoice
	err := session.QueryRowCtx(ctx, &existing, invoiceSelect+` WHERE external_invoice_id=? FOR UPDATE`, externalID)
	if err == nil {
		if existing.TenantId != subscription.TenantId || existing.Currency != currency || existing.TotalAmount != amount ||
			!existing.PeriodStart.Equal(periodStart) || !existing.PeriodEnd.Equal(periodEnd) {
			return nil, status.Error(codes.AlreadyExists, "immutable invoice facts differ from the existing provider invoice")
		}
		paidAmount := existing.PaidAmount
		paidAt := existing.PaidAt
		if targetStatus == core.InvoiceStatus_INVOICE_STATUS_PAID {
			paidAmount = amount
			paidAt = sql.NullTime{Time: billingNow(), Valid: true}
		}
		if _, err := session.ExecCtx(ctx, `UPDATE t_invoice SET status=?,paid_amount=?,paid_at=? WHERE id=?`,
			int64(targetStatus), paidAmount, paidAt, existing.Id); err != nil {
			return nil, billingInternalError("update invoice payment status", err)
		}
		existing.Status, existing.PaidAmount, existing.PaidAt = int64(targetStatus), paidAmount, paidAt
		return &existing, nil
	}
	if err != sqlx.ErrNotFound {
		return nil, billingInternalError("load provider invoice", err)
	}
	subtotal := plan.PriceAmount
	discount, tax := int64(0), int64(0)
	if amount < subtotal {
		discount = subtotal - amount
	} else if amount > subtotal {
		tax = amount - subtotal
	}
	paidAmount := int64(0)
	paidAt := sql.NullTime{}
	if targetStatus == core.InvoiceStatus_INVOICE_STATUS_PAID {
		paidAmount = amount
		paidAt = sql.NullTime{Time: billingNow(), Valid: true}
	}
	result, err := session.ExecCtx(ctx, `INSERT INTO t_invoice
(tenant_id,subscription_id,invoice_no,external_invoice_id,status,currency,subtotal_amount,discount_amount,
tax_amount,total_amount,paid_amount,refunded_amount,period_start,period_end,due_at,paid_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, subscription.TenantId, subscription.Id,
		billingInvoiceNumber(subscription.TenantId, externalID, in.ProviderEventId), externalID, int64(targetStatus),
		currency, subtotal, discount, tax, amount, paidAmount, 0, periodStart, periodEnd,
		sql.NullTime{Time: periodStart.Add(7 * 24 * time.Hour), Valid: true}, paidAt)
	if err != nil {
		return nil, billingInternalError("create immutable invoice", err)
	}
	id, _ := result.LastInsertId()
	if _, err := session.ExecCtx(ctx, `INSERT INTO t_invoice_item
(tenant_id,invoice_id,line_key,item_type,description,metric,quantity,unit_amount,amount,metadata)
VALUES (?,?,?,?,?,NULL,1,?,?,?)`, subscription.TenantId, id, "plan", 1,
		plan.PlanName+" v"+fmt.Sprint(plan.Version), subtotal, subtotal,
		nullString(fmt.Sprintf(`{"planId":%d,"planVersion":%d}`, plan.Id, plan.Version))); err != nil {
		return nil, billingInternalError("create plan invoice item", err)
	}
	if discount > 0 {
		if _, err := session.ExecCtx(ctx, `INSERT INTO t_invoice_item
(tenant_id,invoice_id,line_key,item_type,description,quantity,unit_amount,amount) VALUES (?,?,?,?,?,1,?,?)`,
			subscription.TenantId, id, "discount", 3, "供应商签名折扣", -discount, -discount); err != nil {
			return nil, billingInternalError("create discount invoice item", err)
		}
	}
	if tax > 0 {
		if _, err := session.ExecCtx(ctx, `INSERT INTO t_invoice_item
(tenant_id,invoice_id,line_key,item_type,description,quantity,unit_amount,amount) VALUES (?,?,?,?,?,1,?,?)`,
			subscription.TenantId, id, "tax", 4, "供应商签名税费", tax, tax); err != nil {
			return nil, billingInternalError("create tax invoice item", err)
		}
	}
	var created models.TInvoice
	if err := session.QueryRowCtx(ctx, &created, invoiceSelect+` WHERE id=?`, id); err != nil {
		return nil, billingInternalError("load created invoice", err)
	}
	return &created, nil
}

func insertPaymentTransaction(ctx context.Context, session sqlx.Session, tenant, invoice int64,
	in *core.ApplyBillingWebhookReq, transactionType core.PaymentTransactionType,
	transactionStatus core.PaymentTransactionStatus, occurred time.Time,
) error {
	transactionID := strings.TrimSpace(in.ExternalTransactionId)
	if transactionID == "" {
		transactionID = strings.TrimSpace(in.ProviderEventId)
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "CNY"
	}
	_, err := session.ExecCtx(ctx, `INSERT INTO t_payment_transaction
(tenant_id,invoice_id,provider,provider_transaction_id,transaction_type,status,currency,amount,
failure_code,failure_message,occurred_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE id=id`, tenant, invoice, "stripe", transactionID, int64(transactionType),
		int64(transactionStatus), currency, in.Amount, nullString(in.FailureCode),
		nullString(truncateBillingError(in.FailureMessage)), occurred)
	if err != nil {
		return billingInternalError("record payment transaction", err)
	}
	return nil
}

func findInvoiceByExternalID(ctx context.Context, session sqlx.Session, externalID string) (*models.TInvoice, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return nil, nil
	}
	var item models.TInvoice
	if err := session.QueryRowCtx(ctx, &item, invoiceSelect+` WHERE external_invoice_id=? FOR UPDATE`, externalID); err != nil {
		if err == sqlx.ErrNotFound {
			if lookupErr := session.QueryRowCtx(ctx, &item, invoiceSelect+` WHERE id=(SELECT invoice_id FROM t_payment_transaction
WHERE provider='stripe' AND provider_transaction_id=? AND invoice_id>0 ORDER BY id DESC LIMIT 1) FOR UPDATE`, externalID); lookupErr != nil {
				if lookupErr == sqlx.ErrNotFound {
					return nil, nil
				}
				return nil, billingInternalError("find invoice by payment transaction", lookupErr)
			}
			return &item, nil
		}
		return nil, billingInternalError("find invoice", err)
	}
	return &item, nil
}

func invoiceID(item *models.TInvoice) int64 {
	if item == nil {
		return 0
	}
	return item.Id
}

func truncateBillingError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 1000 {
		return value
	}
	return value[:1000]
}
