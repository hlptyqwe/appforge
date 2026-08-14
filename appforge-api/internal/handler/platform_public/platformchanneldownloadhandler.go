package platform_public

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"appforge/admin-api/internal/logic/platform_public"
	"appforge/admin-api/internal/svc"
	"appforge/common/utils"

	"github.com/zeromicro/go-zero/rest/httpx"
)

const downloadVisitorCookie = "appforge_download_id"

var channelCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

func PlatformChannelDownloadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		channelCode := strings.TrimSpace(r.PathValue("channelCode"))
		if channelCode == "" {
			// go-zero 路由参数由 httpx 写入请求上下文，ParsePath 可兼容该实现。
			var pathReq struct {
				ChannelCode string `path:"channelCode"`
			}
			if err := httpx.ParsePath(r, &pathReq); err == nil {
				channelCode = strings.TrimSpace(pathReq.ChannelCode)
			}
		}
		if !channelCodePattern.MatchString(channelCode) {
			http.Error(w, "invalid channel code", http.StatusBadRequest)
			return
		}

		visitorID := downloadVisitorID(w, r)
		sum := sha256.Sum256([]byte(channelCode + "\x00" + visitorID + "\x00" + time.Now().UTC().Format("2006-01-02")))
		eventKey := hex.EncodeToString(sum[:])
		logic := platform_public.NewPlatformChannelDownloadLogic(r.Context(), svcCtx)
		downloadURL, err := logic.Resolve(channelCode, eventKey, utils.GetClientIP(r), r.UserAgent())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		http.Redirect(w, r, downloadURL, http.StatusFound)
	}
}

func downloadVisitorID(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(downloadVisitorCookie); err == nil && len(cookie.Value) == 32 {
		if _, decodeErr := hex.DecodeString(cookie.Value); decodeErr == nil {
			return cookie.Value
		}
	}
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		fallback := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", utils.GetClientIP(r), time.Now().UnixNano())))
		value = fallback[:16]
	}
	visitorID := hex.EncodeToString(value)
	http.SetCookie(w, &http.Cookie{
		Name: downloadVisitorCookie, Value: visitorID, Path: "/d/", MaxAge: 365 * 24 * 60 * 60,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	})
	return visitorID
}
