package auth_public

import (
	"net/http"

	"appforge/admin-api/internal/logic/auth_public"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/common/utils"
	"appforge/proto/system"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// AgentLoginHandler 固定使用代理端应用范围，避免由请求参数切换身份域。
func AgentLoginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LoginReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		logic := auth_public.NewLoginLogic(r.Context(), svcCtx)
		resp, err := logic.LoginWithScope(
			&req,
			utils.GetClientIP(r),
			system.ApplicationScope_APPLICATION_SCOPE_AGENT,
		)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
