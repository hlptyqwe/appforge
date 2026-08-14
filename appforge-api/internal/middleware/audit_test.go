package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"appforge/admin-api/internal/config"
	"appforge/proto/system"

	"google.golang.org/grpc"
)

type opLogWriterStub struct {
	requests []*system.CreateOpLogReq
}

func (s *opLogWriterStub) CreateOpLog(
	_ context.Context,
	in *system.CreateOpLogReq,
	_ ...grpc.CallOption,
) (*system.RespBase, error) {
	s.requests = append(s.requests, in)
	return &system.RespBase{}, nil
}

func TestAuditMiddlewareRedactsSecretsAndForcesTenantScope(t *testing.T) {
	writer := &opLogWriterStub{}
	middleware := newAuditMiddlewareForTest(t, writer)
	handler := middleware.Handle(func(w http.ResponseWriter, r *http.Request) {
		setAuditActor(r.Context(), 12, "tenant-admin", 7)
		setAuditPermission(r.Context(), "system:user:update")
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"token":"response-secret"}`))
	})

	req := httptest.NewRequest(
		http.MethodPut,
		"/admin/system/users/10?accessToken=query-secret",
		strings.NewReader(`{"tenantId":99,"password":"request-secret","keystorePassword":"keystore-secret","parameterValuesJson":"{\"clientSecret\":\"oauth-plaintext\"}","profile":{"otp":"123456","keyPasswordCiphertext":"cipher-secret"}}`),
	)
	req.RemoteAddr = "192.0.2.1:1234"
	resp := httptest.NewRecorder()
	handler(resp, req)

	if len(writer.requests) != 1 {
		t.Fatalf("expected one audit log, got %d", len(writer.requests))
	}
	log := writer.requests[0]
	if log.TenantId != 7 {
		t.Fatalf("tenant admin target tenant must be forced to 7, got %d", log.TenantId)
	}
	if log.UserId != 12 || log.Username != "tenant-admin" {
		t.Fatalf("unexpected actor: user=%d username=%q", log.UserId, log.Username)
	}
	if log.Module != "system:user" || log.Action != "update" {
		t.Fatalf("unexpected module/action: %q/%q", log.Module, log.Action)
	}
	for _, secret := range []string{"request-secret", "keystore-secret", "cipher-secret", "query-secret", "123456", "response-secret", "oauth-plaintext"} {
		if strings.Contains(log.Req, secret) || strings.Contains(log.Resp, secret) {
			t.Fatalf("secret %q was not redacted: req=%s resp=%s", secret, log.Req, log.Resp)
		}
	}
	if !strings.Contains(log.Req, `"password":"***"`) || !strings.Contains(log.Req, `"keystorePassword":"***"`) || !strings.Contains(log.Req, `"keyPasswordCiphertext":"***"`) || !strings.Contains(log.Resp, `"token":"***"`) {
		t.Fatalf("redaction marker missing: req=%s resp=%s", log.Req, log.Resp)
	}
	if log.Ip != "192.0.2.1" || log.Method != system.RequestMethod_REQUEST_METHOD_PUT {
		t.Fatalf("unexpected request metadata: ip=%q method=%v", log.Ip, log.Method)
	}
}

func TestAuditMiddlewareSystemAdminUsesTargetTenant(t *testing.T) {
	writer := &opLogWriterStub{}
	middleware := newAuditMiddlewareForTest(t, writer)
	handler := middleware.Handle(func(w http.ResponseWriter, r *http.Request) {
		setAuditActor(r.Context(), 1, "admin", 0)
		setAuditPermission(r.Context(), "core:application:create")
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/core/applications", strings.NewReader(`{"tenant_id":99}`))
	handler(httptest.NewRecorder(), req)

	if len(writer.requests) != 1 || writer.requests[0].TenantId != 99 {
		t.Fatalf("system admin target tenant was not recorded: %#v", writer.requests)
	}
	if !strings.Contains(writer.requests[0].Resp, `"httpStatus":204`) {
		t.Fatalf("response status missing: %s", writer.requests[0].Resp)
	}
}

func TestAuditMiddlewareSkipsReadOnlyAndExcludedRequests(t *testing.T) {
	writer := &opLogWriterStub{}
	middleware := newAuditMiddlewareForTest(t, writer)
	handler := middleware.Handle(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/system/users", nil))
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/system/auth/login", nil))
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/channel/event", nil))

	if len(writer.requests) != 0 {
		t.Fatalf("read-only or excluded requests must not be audited, got %d", len(writer.requests))
	}
}

func TestNewAuditMiddlewareRejectsInvalidConfig(t *testing.T) {
	if _, err := NewAuditMiddleware(&opLogWriterStub{}, nil); err == nil {
		t.Fatal("expected empty audit route config to fail")
	}
	if _, err := NewAuditMiddleware(&opLogWriterStub{}, []config.AuditRoute{{
		Method: http.MethodGet,
		Path:   "/admin/system/users",
	}}); err == nil {
		t.Fatal("expected read-only audit route to fail")
	}
}

func newAuditMiddlewareForTest(t *testing.T, writer opLogWriter) *AuditMiddleware {
	t.Helper()
	middleware, err := NewAuditMiddleware(writer, []config.AuditRoute{
		{Method: http.MethodPut, Path: "/admin/system/users/:userId"},
		{Method: http.MethodPost, Path: "/admin/core/applications"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return middleware
}
