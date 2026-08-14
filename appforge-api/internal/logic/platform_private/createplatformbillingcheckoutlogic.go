// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/common/utils"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreatePlatformBillingCheckoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformBillingCheckoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformBillingCheckoutLogic {
	return &CreatePlatformBillingCheckoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformBillingCheckoutLogic) CreatePlatformBillingCheckout(req *types.CreatePlatformBillingCheckoutReq) (resp *types.PlatformBillingCheckoutResp, err error) {
	if req == nil || req.PlanId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "planId is required")
	}
	tenantID, err := utils.GetTenantIdFromCtx(l.ctx)
	if err != nil || tenantID <= 0 {
		return nil, status.Error(codes.PermissionDenied, "tenant checkout context is required")
	}
	planResp, err := l.svcCtx.CoreCli.GetBillingPlan(l.ctx, &core.BillingPlanIdReq{Id: req.PlanId})
	if err != nil || planResp == nil || planResp.Data == nil {
		return nil, err
	}
	plan := planResp.Data
	if plan.Status != core.BillingPlanStatus_BILLING_PLAN_STATUS_ACTIVE {
		return nil, status.Error(codes.FailedPrecondition, "billing plan is retired")
	}
	if plan.PriceAmount == 0 {
		changed, err := l.svcCtx.CoreCli.ChangeTenantSubscription(l.ctx, &core.ChangeTenantSubscriptionReq{
			PlanId: plan.Id, Mode: core.SubscriptionChangeMode_SUBSCRIPTION_CHANGE_MODE_IMMEDIATE,
		})
		if err != nil {
			return nil, err
		}
		return &types.PlatformBillingCheckoutResp{RespBase: platformlogic.PlatformRespBase(changed.Base),
			CheckoutUrl: l.svcCtx.Config.Billing.SuccessURL}, nil
	}
	config := l.svcCtx.Config.Billing
	if strings.TrimSpace(config.StripeSecretKey) == "" {
		return nil, status.Error(codes.Unavailable, "Stripe checkout is not configured")
	}
	apiBase := strings.TrimRight(strings.TrimSpace(config.StripeAPIBaseURL), "/")
	if apiBase == "" {
		apiBase = "https://api.stripe.com"
	}
	interval := "month"
	if plan.BillingCycle == core.BillingCycle_BILLING_CYCLE_YEARLY {
		interval = "year"
	}
	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("success_url", config.SuccessURL)
	form.Set("cancel_url", config.CancelURL)
	form.Set("client_reference_id", strconv.FormatInt(tenantID, 10))
	form.Set("line_items[0][price_data][currency]", strings.ToLower(plan.Currency))
	form.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(plan.PriceAmount, 10))
	form.Set("line_items[0][price_data][product_data][name]", plan.PlanName)
	form.Set("line_items[0][price_data][recurring][interval]", interval)
	form.Set("line_items[0][quantity]", "1")
	for key, value := range map[string]string{
		"tenant_id": strconv.FormatInt(tenantID, 10), "plan_id": strconv.FormatInt(plan.Id, 10),
		"plan_version": strconv.FormatInt(int64(plan.Version), 10),
	} {
		form.Set("metadata["+key+"]", value)
		form.Set("subscription_data[metadata]["+key+"]", value)
	}
	httpReq, err := http.NewRequestWithContext(l.ctx, http.MethodPost, apiBase+"/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, status.Error(codes.Internal, "create Stripe request failed")
	}
	httpReq.SetBasicAuth(config.StripeSecretKey, "")
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Idempotency-Key", fmt.Sprintf("checkout-%d-%d-%d", tenantID, plan.Id, time.Now().Unix()/30))
	httpResp, err := (&http.Client{Timeout: 12 * time.Second}).Do(httpReq)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Stripe checkout is temporarily unavailable")
	}
	defer httpResp.Body.Close()
	var result struct {
		Id    string `json:"id"`
		URL   string `json:"url"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return nil, status.Error(codes.Unavailable, "Stripe returned an invalid response")
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 || result.Id == "" || result.URL == "" {
		message := "Stripe rejected checkout creation"
		if result.Error != nil && strings.TrimSpace(result.Error.Message) != "" {
			message = result.Error.Message
		}
		return nil, status.Error(codes.Unavailable, message)
	}
	return &types.PlatformBillingCheckoutResp{RespBase: types.RespBase{Code: 200, Msg: "OK"},
		CheckoutUrl: result.URL, SessionId: result.Id}, nil
}
