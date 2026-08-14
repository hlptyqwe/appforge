package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"appforge/admin-api/internal/svc"
	"appforge/common/utils"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

const openApiResponseCaptureLimit = 1 << 20

type openApiPrincipalKey struct{}

type OpenApiPrincipal struct {
	CredentialID       int64
	TenantID           int64
	KeyID              string
	Scopes             map[core.OpenApiScope]struct{}
	AppIDs             map[int64]struct{}
	RateLimitPerMinute int32
	RequestID          string
	ScopeUsed          string
	ResourceType       string
	ResourceID         int64
}

type openApiRateWindow struct {
	Minute int64
	Count  int32
}

type OpenApiMiddleware struct {
	svcCtx *svc.ServiceContext
	mu     sync.Mutex
	rates  map[int64]openApiRateWindow
}

func NewOpenApiMiddleware(svcCtx *svc.ServiceContext) *OpenApiMiddleware {
	return &OpenApiMiddleware{svcCtx: svcCtx, rates: make(map[int64]openApiRateWindow)}
}

func OpenApiPrincipalFromContext(ctx context.Context) (*OpenApiPrincipal, bool) {
	value, ok := ctx.Value(openApiPrincipalKey{}).(*OpenApiPrincipal)
	return value, ok && value != nil
}

func RequireOpenApiScope(ctx context.Context, scope core.OpenApiScope, appID int64) bool {
	principal, ok := OpenApiPrincipalFromContext(ctx)
	if !ok {
		return false
	}
	if _, ok := principal.Scopes[scope]; !ok {
		return false
	}
	if appID > 0 && len(principal.AppIDs) > 0 {
		if _, ok := principal.AppIDs[appID]; !ok {
			return false
		}
	}
	principal.ScopeUsed = openApiScopeName(scope)
	return true
}

func SetOpenApiResource(ctx context.Context, resourceType string, resourceID int64) {
	if principal, ok := OpenApiPrincipalFromContext(ctx); ok {
		principal.ResourceType = strings.TrimSpace(resourceType)
		principal.ResourceID = resourceID
	}
}

func (m *OpenApiMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/open/v1/") {
			next(w, r)
			return
		}
		requestID := newOpenApiRequestID()
		w.Header().Set("X-Request-Id", requestID)
		keyID, secret, ok := parseOpenApiKey(r.Header.Get("Authorization"))
		if !ok {
			writeOpenApiError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "invalid API credential")
			return
		}
		digest := sha256.Sum256([]byte(secret))
		result, err := m.svcCtx.CoreCli.AuthenticateOpenApiCredential(r.Context(), &core.AuthenticateOpenApiCredentialReq{
			KeyId: keyID, SecretHash: hex.EncodeToString(digest[:]), ClientIp: requestClientIP(r),
		})
		if err != nil || result == nil || result.Data == nil {
			writeOpenApiError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "invalid or inactive API credential")
			return
		}
		principal := &OpenApiPrincipal{
			CredentialID: result.Data.CredentialId, TenantID: result.Data.TenantId, KeyID: result.Data.KeyId,
			Scopes: make(map[core.OpenApiScope]struct{}, len(result.Data.Scopes)),
			AppIDs: make(map[int64]struct{}, len(result.Data.AppIds)), RateLimitPerMinute: result.Data.RateLimitPerMinute,
			RequestID: requestID,
		}
		for _, scope := range result.Data.Scopes {
			principal.Scopes[scope] = struct{}{}
		}
		for _, appID := range result.Data.AppIds {
			principal.AppIDs[appID] = struct{}{}
		}
		remaining, reset, allowed := m.allow(principal.CredentialID, principal.RateLimitPerMinute)
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(int(principal.RateLimitPerMinute)))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(int(remaining)))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
		if !allowed {
			w.Header().Set("Retry-After", strconv.FormatInt(max(reset-time.Now().Unix(), 0), 10))
			writeOpenApiError(w, http.StatusTooManyRequests, "RATE_LIMITED", "API rate limit exceeded")
			return
		}
		ctx := context.WithValue(r.Context(), openApiPrincipalKey{}, principal)
		ctx = context.WithValue(ctx, utils.CtxKeyTenantId, principal.TenantID)
		ctx = context.WithValue(ctx, utils.CtxKeyUsername, "open:"+principal.KeyID)
		m.handleAuthenticated(w, r.WithContext(ctx), next, principal)
	}
}

func (m *OpenApiMiddleware) handleAuthenticated(w http.ResponseWriter, r *http.Request, next http.HandlerFunc, principal *OpenApiPrincipal) {
	startedAt := time.Now()
	response := &openApiResponseWriter{ResponseWriter: w, status: http.StatusOK, limit: openApiResponseCaptureLimit}
	var idempotencyID int64
	if isOpenApiMutation(r.Method) {
		idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if idempotencyKey == "" {
			writeOpenApiError(response, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required")
			m.recordAudit(r, response, principal, startedAt)
			return
		}
		body, err := readOpenApiBody(r)
		if err != nil {
			writeOpenApiError(response, http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error())
			m.recordAudit(r, response, principal, startedAt)
			return
		}
		digest := sha256.Sum256(append([]byte(r.Method+"\n"+r.URL.RequestURI()+"\n"), body...))
		result, err := m.svcCtx.CoreCli.BeginOpenApiIdempotency(r.Context(), &core.BeginOpenApiIdempotencyReq{
			CredentialId: principal.CredentialID, IdempotencyKey: idempotencyKey,
			RequestMethod: r.Method, RequestPath: r.URL.Path, RequestHash: hex.EncodeToString(digest[:]),
			ExpiresInSeconds: 24 * 60 * 60,
		})
		if err != nil || result == nil || result.Data == nil {
			writeOpenApiError(response, http.StatusServiceUnavailable, "IDEMPOTENCY_UNAVAILABLE", "idempotency service is unavailable")
			m.recordAudit(r, response, principal, startedAt)
			return
		}
		switch result.Data.Decision {
		case core.OpenApiIdempotencyDecision_OPEN_API_IDEMPOTENCY_DECISION_REPLAY:
			response.Header().Set("Idempotent-Replayed", "true")
			response.Header().Set("Content-Type", "application/json")
			response.WriteHeader(int(result.Data.ResponseStatus))
			_, _ = response.Write([]byte(result.Data.ResponseBody))
			m.recordAudit(r, response, principal, startedAt)
			return
		case core.OpenApiIdempotencyDecision_OPEN_API_IDEMPOTENCY_DECISION_IN_PROGRESS:
			writeOpenApiError(response, http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS", "a request with this idempotency key is still processing")
			m.recordAudit(r, response, principal, startedAt)
			return
		case core.OpenApiIdempotencyDecision_OPEN_API_IDEMPOTENCY_DECISION_CONFLICT:
			writeOpenApiError(response, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "the idempotency key was already used for a different request")
			m.recordAudit(r, response, principal, startedAt)
			return
		case core.OpenApiIdempotencyDecision_OPEN_API_IDEMPOTENCY_DECISION_ACQUIRED:
			idempotencyID = result.Data.Id
		default:
			writeOpenApiError(response, http.StatusServiceUnavailable, "IDEMPOTENCY_UNAVAILABLE", "idempotency decision is unavailable")
			m.recordAudit(r, response, principal, startedAt)
			return
		}
	}

	next(response, r)
	if idempotencyID > 0 && response.body.Len() > 0 && json.Valid(response.body.Bytes()) {
		_, err := m.svcCtx.CoreCli.CompleteOpenApiIdempotency(r.Context(), &core.CompleteOpenApiIdempotencyReq{
			Id: idempotencyID, CredentialId: principal.CredentialID,
			ResponseStatus: int32(response.status), ResponseBody: response.body.String(),
			ResourceType: principal.ResourceType, ResourceId: principal.ResourceID,
			Succeeded: response.status >= 200 && response.status < 400,
		})
		if err != nil {
			logx.WithContext(r.Context()).Errorf("complete Open API idempotency failed: requestId=%s err=%v", principal.RequestID, err)
		}
	}
	m.recordAudit(r, response, principal, startedAt)
}

func (m *OpenApiMiddleware) recordAudit(r *http.Request, response *openApiResponseWriter, principal *OpenApiPrincipal, startedAt time.Time) {
	errorCode := ""
	if response.status >= 400 {
		var payload struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(response.body.Bytes(), &payload)
		errorCode = payload.Code
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), time.Second)
	defer cancel()
	_, err := m.svcCtx.CoreCli.RecordOpenApiAudit(ctx, &core.RecordOpenApiAuditReq{
		CredentialId: principal.CredentialID, KeyId: principal.KeyID, RequestId: principal.RequestID,
		RequestMethod: r.Method, RequestPath: r.URL.Path, ScopeUsed: principal.ScopeUsed,
		ResourceType: principal.ResourceType, ResourceId: principal.ResourceID,
		ClientIp: requestClientIP(r), ResponseStatus: int32(response.status),
		DurationMs: time.Since(startedAt).Milliseconds(), ErrorCode: errorCode,
	})
	if err != nil {
		logx.WithContext(r.Context()).Errorf("record Open API audit failed: requestId=%s err=%v", principal.RequestID, err)
	}
}

func (m *OpenApiMiddleware) allow(credentialID int64, limit int32) (remaining int32, reset int64, allowed bool) {
	if limit <= 0 {
		limit = 1
	}
	now := time.Now().Unix()
	minute := now / 60
	reset = (minute + 1) * 60
	m.mu.Lock()
	defer m.mu.Unlock()
	window := m.rates[credentialID]
	if window.Minute != minute {
		window = openApiRateWindow{Minute: minute}
	}
	if window.Count >= limit {
		m.rates[credentialID] = window
		return 0, reset, false
	}
	window.Count++
	m.rates[credentialID] = window
	return limit - window.Count, reset, true
}

func parseOpenApiKey(authorization string) (keyID, secret string, ok bool) {
	parts := strings.Fields(strings.TrimSpace(authorization))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", "", false
	}
	const generatedSecretLength = 43 // 32 random bytes encoded with base64.RawURLEncoding.
	token := strings.TrimPrefix(parts[1], "afk_")
	if token == parts[1] {
		return "", "", false
	}
	// Both key ID and secret use Base64URL and can legally contain underscores.
	// Generated credentials have a fixed-length secret, so split from the right.
	if len(token) > generatedSecretLength && token[len(token)-generatedSecretLength-1] == '_' {
		keyID = token[:len(token)-generatedSecretLength-1]
		secret = token[len(token)-generatedSecretLength:]
		if keyID != "" {
			return keyID, secret, true
		}
	}
	// Keep compatibility with early development credentials that used shorter secrets.
	legacy := strings.SplitN(token, "_", 2)
	if len(legacy) != 2 || legacy[0] == "" || len(legacy[1]) < 32 {
		return "", "", false
	}
	return legacy[0], legacy[1], true
}

func requestClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func openApiScopeName(scope core.OpenApiScope) string {
	names := map[core.OpenApiScope]string{
		core.OpenApiScope_OPEN_API_SCOPE_APPS_READ:      "apps:read",
		core.OpenApiScope_OPEN_API_SCOPE_APPS_WRITE:     "apps:write",
		core.OpenApiScope_OPEN_API_SCOPE_VERSIONS_READ:  "versions:read",
		core.OpenApiScope_OPEN_API_SCOPE_VERSIONS_WRITE: "versions:write",
		core.OpenApiScope_OPEN_API_SCOPE_CHANNELS_READ:  "channels:read",
		core.OpenApiScope_OPEN_API_SCOPE_CHANNELS_WRITE: "channels:write",
		core.OpenApiScope_OPEN_API_SCOPE_BRANDING_READ:  "branding:read",
		core.OpenApiScope_OPEN_API_SCOPE_BRANDING_WRITE: "branding:write",
		core.OpenApiScope_OPEN_API_SCOPE_BUILDS_READ:    "builds:read",
		core.OpenApiScope_OPEN_API_SCOPE_BUILDS_WRITE:   "builds:write",
		core.OpenApiScope_OPEN_API_SCOPE_ARTIFACTS_READ: "artifacts:read",
		core.OpenApiScope_OPEN_API_SCOPE_STATS_READ:     "stats:read",
	}
	return names[scope]
}

func isOpenApiMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func readOpenApiBody(r *http.Request) ([]byte, error) {
	const limit = 2 << 20
	data, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errors.New("request body exceeds 2 MiB")
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return data, nil
}

func newOpenApiRequestID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(value)
}

type openApiResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
	limit  int
}

func (w *openApiResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *openApiResponseWriter) Write(data []byte) (int, error) {
	if w.body.Len() < w.limit {
		remaining := min(w.limit-w.body.Len(), len(data))
		_, _ = w.body.Write(data[:remaining])
	}
	return w.ResponseWriter.Write(data)
}

func writeOpenApiError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "message": message})
}

func WriteOpenApiLogicError(w http.ResponseWriter, err error) {
	httpStatus, body := OpenApiLogicErrorResponse(err)
	writeOpenApiError(w, httpStatus, body["code"].(string), body["message"].(string))
}

func OpenApiLogicErrorResponse(err error) (int, map[string]any) {
	httpStatus := http.StatusBadRequest
	code := "INVALID_ARGUMENT"
	message := err.Error()
	if grpcStatus, ok := grpcstatus.FromError(err); ok {
		message = grpcStatus.Message()
		switch grpcStatus.Code() {
		case codes.InvalidArgument:
			httpStatus, code = http.StatusBadRequest, "INVALID_ARGUMENT"
		case codes.Unauthenticated:
			httpStatus, code = http.StatusUnauthorized, "UNAUTHENTICATED"
		case codes.PermissionDenied:
			httpStatus, code = http.StatusForbidden, "PERMISSION_DENIED"
		case codes.NotFound:
			httpStatus, code = http.StatusNotFound, "NOT_FOUND"
		case codes.AlreadyExists:
			httpStatus, code = http.StatusConflict, "ALREADY_EXISTS"
		case codes.Aborted:
			httpStatus, code = http.StatusConflict, "ABORTED"
		case codes.ResourceExhausted:
			httpStatus, code = http.StatusTooManyRequests, "RESOURCE_EXHAUSTED"
		case codes.FailedPrecondition:
			httpStatus, code = http.StatusUnprocessableEntity, "FAILED_PRECONDITION"
		case codes.Unavailable:
			httpStatus, code = http.StatusServiceUnavailable, "UNAVAILABLE"
		case codes.DeadlineExceeded:
			httpStatus, code = http.StatusGatewayTimeout, "DEADLINE_EXCEEDED"
		default:
			httpStatus, code = http.StatusInternalServerError, "INTERNAL"
		}
	}
	return httpStatus, map[string]any{"code": code, "message": message}
}

func DefaultLogicErrorResponse(err error) (int, any) {
	httpStatus := http.StatusBadRequest
	if grpcStatus, ok := grpcstatus.FromError(err); ok {
		switch grpcStatus.Code() {
		case codes.Unauthenticated:
			httpStatus = http.StatusUnauthorized
		case codes.PermissionDenied:
			httpStatus = http.StatusForbidden
		case codes.NotFound:
			httpStatus = http.StatusNotFound
		case codes.AlreadyExists, codes.Aborted:
			httpStatus = http.StatusConflict
		case codes.ResourceExhausted:
			httpStatus = http.StatusTooManyRequests
		case codes.FailedPrecondition:
			httpStatus = http.StatusPreconditionFailed
		case codes.Unavailable:
			httpStatus = http.StatusServiceUnavailable
		case codes.DeadlineExceeded:
			httpStatus = http.StatusGatewayTimeout
		case codes.Internal, codes.Unknown, codes.DataLoss:
			httpStatus = http.StatusInternalServerError
		}
	}
	return httpStatus, err
}
