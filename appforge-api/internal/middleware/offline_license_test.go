package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"appforge/common/offlinelicense"
)

func TestOfflineLicenseMiddlewareFailsClosedAfterExpiry(t *testing.T) {
	now := time.Now()
	middleware := NewOfflineLicenseMiddleware(&offlinelicense.VerifiedLicense{Payload: offlinelicense.Payload{
		NotBefore: now.Add(-time.Hour).UnixMilli(), NotAfter: now.Add(time.Minute).UnixMilli(),
	}})
	middleware.now = func() time.Time { return now.Add(2 * time.Minute) }
	handler := middleware.Handle(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	response := httptest.NewRecorder()
	handler(response, httptest.NewRequest(http.MethodGet, "/admin/system/users", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expired license status = %d", response.Code)
	}

	liveness := httptest.NewRecorder()
	handler(liveness, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if liveness.Code != http.StatusNoContent {
		t.Fatalf("liveness must remain available, got %d", liveness.Code)
	}

	deploymentStatus := httptest.NewRecorder()
	handler(deploymentStatus, httptest.NewRequest(http.MethodGet, "/admin/core/enterprise/deployment", nil))
	if deploymentStatus.Code != http.StatusNoContent {
		t.Fatalf("authenticated deployment status must remain available, got %d", deploymentStatus.Code)
	}
}

func TestOfflineLicenseMiddlewareAllowsDisabledOrValidLicense(t *testing.T) {
	handler := NewOfflineLicenseMiddleware(nil).Handle(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	response := httptest.NewRecorder()
	handler(response, httptest.NewRequest(http.MethodGet, "/agent/core/applications", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("disabled license status = %d", response.Code)
	}
}

func TestLicenseExemptPathIsExact(t *testing.T) {
	if !licenseExemptPath("/admin/core/enterprise/deployment") {
		t.Fatal("deployment status must be exempt so an expired license remains diagnosable")
	}
	if licenseExemptPath("/admin/core/enterprise/deployment/anything") {
		t.Fatal("license exemption must not match a path prefix")
	}
}
