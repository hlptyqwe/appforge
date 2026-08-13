// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_public

import (
	"net/http"

	"appforge/admin-api/internal/logic/platform_public"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func PlatformInstallReportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PlatformInstallReportReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := platform_public.NewPlatformInstallReportLogic(r.Context(), svcCtx)
		resp, err := l.PlatformInstallReport(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
