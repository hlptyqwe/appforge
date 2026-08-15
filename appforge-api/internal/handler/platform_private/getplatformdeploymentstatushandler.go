// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"net/http"

	"appforge/admin-api/internal/logic/platform_private"
	"appforge/admin-api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetPlatformDeploymentStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := platform_private.NewGetPlatformDeploymentStatusLogic(r.Context(), svcCtx)
		resp, err := l.GetPlatformDeploymentStatus()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
