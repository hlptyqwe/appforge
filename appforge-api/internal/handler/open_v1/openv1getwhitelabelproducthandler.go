// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"net/http"

	"appforge/admin-api/internal/logic/open_v1"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func OpenV1GetWhiteLabelProductHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PlatformIdReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := open_v1.NewOpenV1GetWhiteLabelProductLogic(r.Context(), svcCtx)
		resp, err := l.OpenV1GetWhiteLabelProduct(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
