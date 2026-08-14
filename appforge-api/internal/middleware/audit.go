package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"appforge/admin-api/internal/config"
	"appforge/common/siem"
	"appforge/common/utils"
	"appforge/proto/system"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
)

const auditCaptureLimit = 64 << 10

type opLogWriter interface {
	CreateOpLog(ctx context.Context, in *system.CreateOpLogReq, opts ...grpc.CallOption) (*system.RespBase, error)
}

type AuditMiddleware struct {
	writer   opLogWriter
	routes   []auditRouteSpec
	exporter auditSIEMExporter
}

type auditSIEMExporter interface {
	Export(event siem.Event) bool
}

type auditContextKey struct{}

type auditState struct {
	userID     int64
	tenantID   int64
	username   string
	permission string
}

func NewAuditMiddleware(writer opLogWriter, configuredRoutes []config.AuditRoute, exporters ...auditSIEMExporter) (*AuditMiddleware, error) {
	routes, err := buildSensitiveAuditRoutes(configuredRoutes)
	if err != nil {
		return nil, err
	}
	var exporter auditSIEMExporter
	if len(exporters) > 0 {
		exporter = exporters[0]
	}
	return &AuditMiddleware{writer: writer, routes: routes, exporter: exporter}, nil
}

func (m *AuditMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.writer == nil || !m.shouldAuditRequest(r) {
			next(w, r)
			return
		}

		state := &auditState{}
		state.userID, _ = utils.GetUserIdFromCtx(r.Context())
		state.username, _ = utils.GetUsernameFromCtx(r.Context())
		ctx := context.WithValue(r.Context(), auditContextKey{}, state)
		r = r.WithContext(ctx)

		body := &auditBodyCapture{ReadCloser: r.Body, limit: auditCaptureLimit}
		if r.Body != nil {
			r.Body = body
		}
		response := &auditResponseWriter{ResponseWriter: w, status: http.StatusOK, limit: auditCaptureLimit}
		startedAt := time.Now()

		next(response, r)

		module, action := auditModuleAction(state.permission, r.URL.Path, r.Method)
		tenantID := state.tenantID
		if tenantID == 0 {
			tenantID = auditTargetTenant(r.URL.Query(), body.Bytes())
		}

		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), time.Second)
		defer cancel()
		writeCtx = context.WithValue(writeCtx, utils.CtxKeyUid, state.userID)
		writeCtx = context.WithValue(writeCtx, utils.CtxKeyUsername, state.username)
		writeCtx = context.WithValue(writeCtx, utils.CtxKeyTenantId, state.tenantID)

		entry := &system.CreateOpLogReq{
			TenantId: tenantID,
			UserId:   state.userID,
			Username: state.username,
			Module:   module,
			Action:   action,
			Method:   auditRequestMethod(r.Method),
			Path:     r.URL.Path,
			Req:      auditRequestSummary(r.URL.Query(), body.Bytes(), body.truncated),
			Resp:     auditResponseSummary(response.status, response.Bytes(), response.truncated),
			Ip:       utils.GetClientIP(r),
			CostMs:   time.Since(startedAt).Milliseconds(),
		}
		_, err := m.writer.CreateOpLog(writeCtx, entry)
		if err != nil {
			logx.WithContext(r.Context()).Errorf("create operation audit log failed: method=%s path=%s err=%v", r.Method, r.URL.Path, err)
		}
		if m.exporter != nil && !m.exporter.Export(siem.Event{
			Timestamp: startedAt.UnixMilli(), TenantID: entry.TenantId, UserID: entry.UserId, Username: entry.Username,
			Module: entry.Module, Action: entry.Action, Method: r.Method, Path: entry.Path,
			Request: entry.Req, Response: entry.Resp, IP: entry.Ip, CostMS: entry.CostMs,
		}) {
			logx.WithContext(r.Context()).Errorf("SIEM audit queue is full: method=%s path=%s", r.Method, r.URL.Path)
		}
	}
}

func setAuditActor(ctx context.Context, userID int64, username string, tenantID int64) {
	if state, ok := ctx.Value(auditContextKey{}).(*auditState); ok {
		state.userID = userID
		state.username = username
		state.tenantID = tenantID
	}
}

func setAuditPermission(ctx context.Context, permission string) {
	if state, ok := ctx.Value(auditContextKey{}).(*auditState); ok {
		state.permission = permission
	}
}

func (m *AuditMiddleware) shouldAuditRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	for _, route := range m.routes {
		if route.method == r.Method && route.pattern.MatchString(normalizePath(r.URL.Path)) {
			return true
		}
	}
	return false
}

func auditModuleAction(permission, path, method string) (string, string) {
	if obj, act, ok := parsePerm(permission); ok {
		return obj, act
	}
	path = strings.TrimPrefix(path, "/admin/")
	path = strings.TrimPrefix(path, "/agent/")
	path = strings.Trim(path, "/")
	module := strings.Split(path, "/")[0]
	if module == "" {
		module = "unknown"
	}
	return module, strings.ToLower(method)
}

func auditRequestMethod(method string) system.RequestMethod {
	switch method {
	case http.MethodGet:
		return system.RequestMethod_REQUEST_METHOD_GET
	case http.MethodPost:
		return system.RequestMethod_REQUEST_METHOD_POST
	case http.MethodPut:
		return system.RequestMethod_REQUEST_METHOD_PUT
	case http.MethodDelete:
		return system.RequestMethod_REQUEST_METHOD_DELETE
	default:
		return system.RequestMethod_REQUEST_METHOD_UNKNOWN
	}
}

type auditBodyCapture struct {
	io.ReadCloser
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (c *auditBodyCapture) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	if remaining := c.limit - c.buf.Len(); n > 0 && remaining > 0 {
		copyLen := n
		if copyLen > remaining {
			copyLen = remaining
		}
		_, _ = c.buf.Write(p[:copyLen])
	}
	if c.buf.Len() >= c.limit && n > 0 {
		c.truncated = true
	}
	return n, err
}

func (c *auditBodyCapture) Bytes() []byte { return c.buf.Bytes() }

type auditResponseWriter struct {
	http.ResponseWriter
	buf         bytes.Buffer
	status      int
	limit       int
	wroteHeader bool
	truncated   bool
}

func (w *auditResponseWriter) WriteHeader(statusCode int) {
	if !w.wroteHeader {
		w.status = statusCode
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *auditResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if remaining := w.limit - w.buf.Len(); remaining > 0 {
		copyLen := len(p)
		if copyLen > remaining {
			copyLen = remaining
		}
		_, _ = w.buf.Write(p[:copyLen])
	}
	if w.buf.Len() >= w.limit && len(p) > 0 {
		w.truncated = true
	}
	return w.ResponseWriter.Write(p)
}

func (w *auditResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *auditResponseWriter) Bytes() []byte               { return w.buf.Bytes() }

func auditRequestSummary(query url.Values, body []byte, truncated bool) string {
	data := map[string]any{}
	if len(query) > 0 {
		data["query"] = redactAuditValue(valuesToMap(query))
	}
	if len(bytes.TrimSpace(body)) > 0 {
		data["body"] = decodeAndRedactAuditJSON(body)
	}
	if truncated {
		data["truncated"] = true
	}
	return marshalAuditSummary(data)
}

func auditResponseSummary(status int, body []byte, truncated bool) string {
	data := map[string]any{"httpStatus": status}
	if len(bytes.TrimSpace(body)) > 0 {
		data["body"] = decodeAndRedactAuditJSON(body)
	}
	if truncated {
		data["truncated"] = true
	}
	return marshalAuditSummary(data)
}

func decodeAndRedactAuditJSON(data []byte) any {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		// Audit logs are not a payload archive. Keeping malformed or non-JSON
		// bodies would allow credentials to bypass the structured redactor.
		return "[REDACTED_NON_JSON_BODY]"
	}
	return redactAuditValue(value)
}

func redactAuditValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if isAuditSecretKey(key) {
				v[key] = "***"
				continue
			}
			v[key] = redactAuditValue(item)
		}
		return v
	case []any:
		for i := range v {
			v[i] = redactAuditValue(v[i])
		}
		return v
	case string:
		// Some API fields contain JSON encoded as a string. Redact the nested
		// structure too, while preserving ordinary non-JSON strings.
		trimmed := strings.TrimSpace(v)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			var nested any
			decoder := json.NewDecoder(strings.NewReader(trimmed))
			decoder.UseNumber()
			if decoder.Decode(&nested) == nil {
				redacted, err := json.Marshal(redactAuditValue(nested))
				if err == nil {
					return string(redacted)
				}
			}
		}
		return value
	default:
		return value
	}
}

func isAuditSecretKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(strings.TrimSpace(key)))
	// Product-specific fields such as keystorePassword, keyPassword,
	// tenantPassword and passwordCiphertext must be covered as well as the
	// generic JSON names. Matching semantic fragments prevents a new password
	// field from silently becoming audit-log plaintext.
	for _, fragment := range []string{"password", "privatekey", "ciphertext", "mnemonic"} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	for _, suffix := range []string{"token", "secret"} {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	switch normalized {
	case "pwd", "token", "accesstoken", "refreshtoken",
		"authorization", "cookie", "secret", "apikey", "accesskey", "secretkey",
		"sessionkey", "googlecode", "otp", "verificationcode", "parametervaluesjson",
		"webhookurl", "callbackurl", "downloadurl", "uploadurl", "presignedurl":
		return true
	default:
		return false
	}
}

func valuesToMap(values url.Values) map[string]any {
	result := make(map[string]any, len(values))
	for key, items := range values {
		if len(items) == 1 {
			result[key] = items[0]
		} else {
			result[key] = items
		}
	}
	return result
}

func auditTargetTenant(query url.Values, body []byte) int64 {
	for _, key := range []string{"tenantId", "tenant_id"} {
		if id, err := strconv.ParseInt(strings.TrimSpace(query.Get(key)), 10, 64); err == nil && id > 0 {
			return id
		}
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if decoder.Decode(&value) == nil {
		return findAuditTenant(value)
	}
	return 0
}

func findAuditTenant(value any) int64 {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			normalized := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(key))
			if normalized == "tenantid" {
				switch id := item.(type) {
				case json.Number:
					value, _ := id.Int64()
					if value > 0 {
						return value
					}
				case string:
					value, _ := strconv.ParseInt(strings.TrimSpace(id), 10, 64)
					if value > 0 {
						return value
					}
				}
			}
			if id := findAuditTenant(item); id > 0 {
				return id
			}
		}
	case []any:
		for _, item := range v {
			if id := findAuditTenant(item); id > 0 {
				return id
			}
		}
	}
	return 0
}

func marshalAuditSummary(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return truncateAuditString(string(data), 8192)
}

func truncateAuditString(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
