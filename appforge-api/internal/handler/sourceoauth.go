package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"appforge/admin-api/internal/sourceoauth"
	"appforge/admin-api/internal/svc"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterSourceOAuthHandlers registers the public callback. Tenant and actor
// identity come only from the signed, short-lived OAuth state.
func RegisterSourceOAuthHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/public/v1/source-oauth/callback", Handler: func(writer http.ResponseWriter, request *http.Request) {
		state := request.URL.Query().Get("state")
		platform := sourceoauth.PlatformFromState(svcCtx.Config.Jwt.AccessSecret, state)
		succeeded := false
		if request.URL.Query().Get("error") == "" {
			succeeded = sourceoauth.Complete(request.Context(), svcCtx, request.URL.Query().Get("code"), state) == nil
		}
		http.Redirect(writer, request, sourceoauth.RedirectURL(svcCtx.Config.SourceOAuth, platform, succeeded), http.StatusSeeOther)
	}})
	server.AddRoute(rest.Route{Method: http.MethodPost, Path: "/public/v1/source-webhooks/:platform/:token", Handler: func(writer http.ResponseWriter, request *http.Request) {
		var pathReq struct {
			Platform string `path:"platform"`
			Token    string `path:"token"`
		}
		// Parse only route parameters. httpx.Parse also consumes the JSON body,
		// which would make signature verification operate on an empty payload.
		if err := httpx.ParsePath(request, &pathReq); err != nil {
			httpx.ErrorCtx(request.Context(), writer, err)
			return
		}
		platform := core.SourcePlatform_SOURCE_PLATFORM_UNKNOWN
		switch strings.ToLower(strings.TrimSpace(pathReq.Platform)) {
		case "github":
			platform = core.SourcePlatform_SOURCE_PLATFORM_GITHUB
		case "gitlab":
			platform = core.SourcePlatform_SOURCE_PLATFORM_GITLAB
		}
		body, err := io.ReadAll(io.LimitReader(request.Body, (2<<20)+1))
		if err != nil {
			httpx.ErrorCtx(request.Context(), writer, err)
			return
		}
		result, err := sourceoauth.ReceiveSourceWebhook(request.Context(), svcCtx, platform, pathReq.Token, request.Header, body)
		if err != nil {
			httpx.ErrorCtx(request.Context(), writer, err)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(http.StatusAccepted)
		message := "accepted"
		if result.Ignored {
			message = "ignored"
		} else if !result.Accepted {
			message = "duplicate"
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"code": http.StatusAccepted, "msg": message, "eventId": result.EventID})
	}})
}
