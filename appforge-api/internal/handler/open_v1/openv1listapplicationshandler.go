// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"net/http"

	"appforge/admin-api/internal/logic/open_v1"
	"appforge/admin-api/internal/middleware"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func OpenV1ListApplicationsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OpenListApplicationsReq
		if err := httpx.Parse(r, &req); err != nil {
			middleware.WriteOpenApiLogicError(w, err)
			return
		}

		l := open_v1.NewOpenV1ListApplicationsLogic(r.Context(), svcCtx)
		resp, err := l.OpenV1ListApplications(&req)
		if err != nil {
			middleware.WriteOpenApiLogicError(w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
