package middleware

import (
	"encoding/json"
	"net/http"
	"time"

	"appforge/common/offlinelicense"
)

type OfflineLicenseMiddleware struct {
	license *offlinelicense.VerifiedLicense
	now     func() time.Time
}

func NewOfflineLicenseMiddleware(license *offlinelicense.VerifiedLicense) *OfflineLicenseMiddleware {
	return &OfflineLicenseMiddleware{license: license, now: time.Now}
}

func (m *OfflineLicenseMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if licenseExemptPath(r.URL.Path) || m.license == nil {
			next(w, r)
			return
		}
		if err := m.license.ValidAt(m.now()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 503, "msg": "enterprise license is not valid"})
			return
		}
		next(w, r)
	}
}

func licenseExemptPath(path string) bool {
	switch path {
	case "/healthz", "/readyz", "/admin/core/enterprise/deployment":
		return true
	default:
		return false
	}
}
