// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"net/http"

	"appforge/admin-api/internal/logic/platform_private"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func CreatePlatformBillingPlanHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreatePlatformBillingPlanReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := platform_private.NewCreatePlatformBillingPlanLogic(r.Context(), svcCtx)
		resp, err := l.CreatePlatformBillingPlan(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
