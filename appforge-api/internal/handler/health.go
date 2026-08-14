package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"appforge/admin-api/internal/svc"
)

// HealthHandler reports process liveness. Configuration loading and service
// context initialization finish before this route is registered, so a 200
// response also proves that mandatory startup validation completed.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ReadyHandler additionally verifies time-bound enterprise license readiness.
func ReadyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if svcCtx != nil && svcCtx.License != nil {
			if err := svcCtx.License.ValidAt(time.Now()); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "not_ready", "reason": "enterprise_license_invalid"})
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}
}
