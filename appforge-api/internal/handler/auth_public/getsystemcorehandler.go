// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package auth_public

import (
	"net/http"

	"appforge/admin-api/internal/logic/auth_public"
	"appforge/admin-api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetSystemCoreHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := auth_public.NewGetSystemCoreLogic(r.Context(), svcCtx)
		resp, err := l.GetSystemCore()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
