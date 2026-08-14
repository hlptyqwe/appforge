package logic

import (
	"context"
	"encoding/json"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	openApiIdempotencyProcessing = 1
	openApiIdempotencyCompleted  = 2
	openApiIdempotencyFailed     = 3
)

func beginOpenApiIdempotency(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BeginOpenApiIdempotencyReq) (*core.OpenApiIdempotencyResp, error) {
	if in == nil || in.CredentialId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "credential_id is required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	method := strings.ToUpper(strings.TrimSpace(in.RequestMethod))
	path := strings.TrimSpace(in.RequestPath)
	hash := strings.ToLower(strings.TrimSpace(in.RequestHash))
	if key == "" || len(key) > 128 {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key must be between 1 and 128 characters")
	}
	if method == "" || len(method) > 16 || path == "" || len(path) > 255 || len(hash) != 64 {
		return nil, status.Error(codes.InvalidArgument, "invalid idempotency request metadata")
	}
	expires := in.ExpiresInSeconds
	if expires <= 0 {
		expires = 24 * 60 * 60
	}
	if expires > 7*24*60*60 {
		expires = 7 * 24 * 60 * 60
	}
	result := &core.OpenApiIdempotencyResult{}
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		reset, updateErr := session.ExecCtx(txCtx, `UPDATE t_open_api_idempotency
SET request_hash = ?, response_status = 0, response_body = NULL, resource_type = NULL, resource_id = 0,
status = ?, expires_at = DATE_ADD(CURRENT_TIMESTAMP(3), INTERVAL ? SECOND), update_time = CURRENT_TIMESTAMP(3)
WHERE tenant_id = ? AND credential_id = ? AND request_method = ? AND request_path = ? AND idempotency_key = ?
AND expires_at <= CURRENT_TIMESTAMP(3)`, hash, openApiIdempotencyProcessing, expires, tenant, in.CredentialId, method, path, key)
		if updateErr != nil {
			return updateErr
		}
		if affected, _ := reset.RowsAffected(); affected == 1 {
			if err := session.QueryRowCtx(txCtx, &result.Id, `SELECT id FROM t_open_api_idempotency
WHERE tenant_id = ? AND credential_id = ? AND request_method = ? AND request_path = ? AND idempotency_key = ?`,
				tenant, in.CredentialId, method, path, key); err != nil {
				return err
			}
			result.Decision = core.OpenApiIdempotencyDecision_OPEN_API_IDEMPOTENCY_DECISION_ACQUIRED
			return nil
		}
		inserted, insertErr := session.ExecCtx(txCtx, `INSERT IGNORE INTO t_open_api_idempotency
(tenant_id, credential_id, idempotency_key, request_method, request_path, request_hash, response_status,
response_body, resource_type, resource_id, status, expires_at)
VALUES (?, ?, ?, ?, ?, ?, 0, NULL, NULL, 0, ?, DATE_ADD(CURRENT_TIMESTAMP(3), INTERVAL ? SECOND))`,
			tenant, in.CredentialId, key, method, path, hash, openApiIdempotencyProcessing, expires)
		if insertErr != nil {
			return insertErr
		}
		if affected, _ := inserted.RowsAffected(); affected == 1 {
			result.Id, _ = inserted.LastInsertId()
			result.Decision = core.OpenApiIdempotencyDecision_OPEN_API_IDEMPOTENCY_DECISION_ACQUIRED
			return nil
		}
		var stored struct {
			Id             int64  `db:"id"`
			RequestHash    string `db:"request_hash"`
			Status         int64  `db:"status"`
			ResponseStatus int64  `db:"response_status"`
			ResponseBody   string `db:"response_body"`
		}
		if err := session.QueryRowCtx(txCtx, &stored,
			`SELECT id, request_hash, status, response_status, COALESCE(response_body, '') AS response_body
FROM t_open_api_idempotency WHERE tenant_id = ? AND credential_id = ? AND request_method = ?
AND request_path = ? AND idempotency_key = ? FOR UPDATE`, tenant, in.CredentialId, method, path, key); err != nil {
			return err
		}
		result.Id = stored.Id
		switch {
		case stored.RequestHash != hash:
			result.Decision = core.OpenApiIdempotencyDecision_OPEN_API_IDEMPOTENCY_DECISION_CONFLICT
		case stored.Status == openApiIdempotencyCompleted || stored.Status == openApiIdempotencyFailed:
			result.Decision = core.OpenApiIdempotencyDecision_OPEN_API_IDEMPOTENCY_DECISION_REPLAY
			result.ResponseStatus = int32(stored.ResponseStatus)
			result.ResponseBody = stored.ResponseBody
		default:
			result.Decision = core.OpenApiIdempotencyDecision_OPEN_API_IDEMPOTENCY_DECISION_IN_PROGRESS
		}
		return nil
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "begin idempotency request failed: %v", err)
	}
	return &core.OpenApiIdempotencyResp{Base: okBase(), Data: result}, nil
}

func completeOpenApiIdempotency(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CompleteOpenApiIdempotencyReq) (*core.RespBase, error) {
	if in == nil || in.Id <= 0 || in.CredentialId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id and credential_id are required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(in.ResponseBody)
	if body == "" || len(body) > 1<<20 || !json.Valid([]byte(body)) {
		return nil, status.Error(codes.InvalidArgument, "response_body must be valid JSON no larger than 1 MiB")
	}
	finalStatus := openApiIdempotencyFailed
	if in.Succeeded {
		finalStatus = openApiIdempotencyCompleted
	}
	result, err := svcCtx.DB.ExecCtx(ctx, `UPDATE t_open_api_idempotency SET response_status = ?,
response_body = CAST(? AS JSON), resource_type = NULLIF(?, ''), resource_id = ?, status = ?, update_time = CURRENT_TIMESTAMP(3)
WHERE id = ? AND tenant_id = ? AND credential_id = ? AND status = ?`, in.ResponseStatus, body,
		strings.TrimSpace(in.ResourceType), in.ResourceId, finalStatus, in.Id, tenant, in.CredentialId, openApiIdempotencyProcessing)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "complete idempotency request failed: %v", err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, status.Error(codes.FailedPrecondition, "idempotency request is no longer processing")
	}
	return &core.RespBase{Base: okBase()}, nil
}

func recordOpenApiAudit(ctx context.Context, svcCtx *svc.ServiceContext, in *core.RecordOpenApiAuditReq) (*core.RespBase, error) {
	if in == nil || in.CredentialId <= 0 || strings.TrimSpace(in.RequestId) == "" {
		return nil, status.Error(codes.InvalidArgument, "credential_id and request_id are required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if len(in.RequestId) > 64 || len(in.RequestPath) > 255 || len(in.ErrorCode) > 64 {
		return nil, status.Error(codes.InvalidArgument, "audit metadata is too long")
	}
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		result, err := session.ExecCtx(txCtx, `INSERT IGNORE INTO t_open_api_audit
(tenant_id, credential_id, key_id, request_id, request_method, request_path, scope_used, resource_type,
resource_id, client_ip, response_status, duration_ms, error_code)
VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''))`,
			tenant, in.CredentialId, strings.TrimSpace(in.KeyId), strings.TrimSpace(in.RequestId),
			strings.ToUpper(strings.TrimSpace(in.RequestMethod)), strings.TrimSpace(in.RequestPath),
			strings.TrimSpace(in.ScopeUsed), strings.TrimSpace(in.ResourceType), in.ResourceId,
			strings.TrimSpace(in.ClientIp), in.ResponseStatus, in.DurationMs, strings.TrimSpace(in.ErrorCode))
		if err != nil {
			return err
		}
		inserted, _ := result.RowsAffected()
		if inserted == 0 {
			return nil
		}
		return insertUsageLedger(txCtx, session, tenant, "api.requests", 1, "open_api_request",
			in.CredentialId, "open-api:"+strings.TrimSpace(in.RequestId), billingNow(),
			billingUsageMetadata(map[string]any{"method": strings.ToUpper(strings.TrimSpace(in.RequestMethod)),
				"path": strings.TrimSpace(in.RequestPath), "responseStatus": in.ResponseStatus}))
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "record Open API audit failed: %v", err)
	}
	return &core.RespBase{Base: okBase()}, nil
}
