// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package auth_public

import (
	"net/http"

	"appforge/admin-api/internal/logic/auth_public"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/common/utils"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func LoginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LoginReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		ip := utils.GetClientIP(r)
		l := auth_public.NewLoginLogic(r.Context(), svcCtx)
		resp, err := l.Login(&req, ip)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
