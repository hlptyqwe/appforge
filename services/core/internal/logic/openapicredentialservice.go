package logic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strings"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const openApiCredentialSelect = `SELECT id,tenant_id,credential_name,key_id,secret_hash,scopes,app_ids,
ip_allowlist,rate_limit_per_minute,status,expires_at,grace_expires_at,rotated_from_id,last_used_at,
create_by,create_time,update_time FROM t_open_api_credential`

var openApiScopeNames = map[core.OpenApiScope]string{
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

func generateOpenApiKey() (keyID, secret, fullKey, secretHash string, err error) {
	keyBytes := make([]byte, 9)
	secretBytes := make([]byte, 32)
	if _, err = rand.Read(keyBytes); err != nil {
		return "", "", "", "", err
	}
	if _, err = rand.Read(secretBytes); err != nil {
		return "", "", "", "", err
	}
	keyID = base64.RawURLEncoding.EncodeToString(keyBytes)
	secret = base64.RawURLEncoding.EncodeToString(secretBytes)
	digest := sha256.Sum256([]byte(secret))
	secretHash = hex.EncodeToString(digest[:])
	fullKey = "afk_" + keyID + "_" + secret
	return
}

func normalizedOpenApiScopes(values []core.OpenApiScope) ([]core.OpenApiScope, string, error) {
	seen := make(map[core.OpenApiScope]struct{}, len(values))
	result := make([]core.OpenApiScope, 0, len(values))
	names := make([]string, 0, len(values))
	for _, value := range values {
		name, ok := openApiScopeNames[value]
		if !ok {
			return nil, "", status.Error(codes.InvalidArgument, "open API scope is invalid")
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
		names = append(names, name)
	}
	if len(result) == 0 {
		return nil, "", status.Error(codes.InvalidArgument, "at least one open API scope is required")
	}
	encoded, _ := json.Marshal(names)
	return result, string(encoded), nil
}

func parseOpenApiScopes(raw string) []core.OpenApiScope {
	var names []string
	if json.Unmarshal([]byte(raw), &names) != nil {
		return nil
	}
	reverse := make(map[string]core.OpenApiScope, len(openApiScopeNames))
	for value, name := range openApiScopeNames {
		reverse[name] = value
	}
	result := make([]core.OpenApiScope, 0, len(names))
	for _, name := range names {
		if value, ok := reverse[name]; ok {
			result = append(result, value)
		}
	}
	return result
}

func normalizedIDJSON(values []int64) ([]int64, string, error) {
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			return nil, "", status.Error(codes.InvalidArgument, "application scope contains invalid id")
		}
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	encoded, _ := json.Marshal(result)
	return result, string(encoded), nil
}

func normalizedIPAllowlist(values []string) ([]string, string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if addr, err := netip.ParseAddr(value); err == nil {
			value = addr.String()
		} else if prefix, err := netip.ParsePrefix(value); err == nil {
			value = prefix.Masked().String()
		} else {
			return nil, "", status.Error(codes.InvalidArgument, "IP allowlist contains invalid address or CIDR")
		}
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	encoded, _ := json.Marshal(result)
	return result, string(encoded), nil
}

func parseInt64JSON(raw string) []int64 {
	var result []int64
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}

func parseStringJSON(raw string) []string {
	var result []string
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}

func mapOpenApiCredential(item *models.TOpenApiCredential) *core.OpenApiCredential {
	if item == nil {
		return nil
	}
	return &core.OpenApiCredential{
		Id: item.Id, TenantId: item.TenantId, CredentialName: item.CredentialName, KeyId: item.KeyId,
		Scopes: parseOpenApiScopes(item.Scopes), AppIds: parseInt64JSON(item.AppIds),
		IpAllowlist: parseStringJSON(item.IpAllowlist), RateLimitPerMinute: int32(item.RateLimitPerMinute),
		Status: core.OpenApiCredentialStatus(item.Status), ExpiresAt: timeValue(item.ExpiresAt),
		GraceExpiresAt: timeValue(item.GraceExpiresAt), RotatedFromId: item.RotatedFromId,
		LastUsedAt: timeValue(item.LastUsedAt), CreateBy: item.CreateBy,
		CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime),
	}
}

func validateCredentialApplications(ctx context.Context, svcCtx *svc.ServiceContext, tenant int64, appIDs []int64) error {
	for _, appID := range appIDs {
		item, err := svcCtx.ApplicationModel.FindOne(ctx, appID)
		if err != nil || item.TenantId != tenant {
			return status.Error(codes.InvalidArgument, "credential application does not belong to tenant")
		}
	}
	return nil
}

func createOpenApiCredential(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CreateOpenApiCredentialReq) (*core.OpenApiCredentialSecretResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.CredentialName)
	if len(name) < 2 || len(name) > 128 {
		return nil, status.Error(codes.InvalidArgument, "credential_name length must be between 2 and 128")
	}
	_, scopesJSON, err := normalizedOpenApiScopes(in.Scopes)
	if err != nil {
		return nil, err
	}
	appIDs, appIDsJSON, err := normalizedIDJSON(in.AppIds)
	if err != nil {
		return nil, err
	}
	if err := validateCredentialApplications(ctx, svcCtx, tenant, appIDs); err != nil {
		return nil, err
	}
	_, allowlistJSON, err := normalizedIPAllowlist(in.IpAllowlist)
	if err != nil {
		return nil, err
	}
	if in.RateLimitPerMinute <= 0 || in.RateLimitPerMinute > 10000 {
		return nil, status.Error(codes.InvalidArgument, "rate_limit_per_minute must be between 1 and 10000")
	}
	var expiresAt sql.NullTime
	if in.ExpiresAt > 0 {
		expiresAt = sql.NullTime{Time: time.UnixMilli(in.ExpiresAt), Valid: true}
		if !expiresAt.Time.After(time.Now()) {
			return nil, status.Error(codes.InvalidArgument, "expires_at must be in the future")
		}
	}
	keyID, _, fullKey, secretHash, err := generateOpenApiKey()
	if err != nil {
		return nil, status.Error(codes.Internal, "generate API credential failed")
	}
	result, err := svcCtx.OpenApiCredentialModel.Insert(ctx, &models.TOpenApiCredential{
		TenantId: tenant, CredentialName: name, KeyId: keyID, SecretHash: secretHash,
		Scopes: scopesJSON, AppIds: appIDsJSON, IpAllowlist: allowlistJSON,
		RateLimitPerMinute: int64(in.RateLimitPerMinute), Status: int64(core.OpenApiCredentialStatus_OPEN_API_CREDENTIAL_STATUS_ACTIVE),
		ExpiresAt: expiresAt, CreateBy: actorID(ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create API credential failed: %v", err)
	}
	id, _ := result.LastInsertId()
	item, err := svcCtx.OpenApiCredentialModel.FindOne(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read created API credential failed: %v", err)
	}
	return &core.OpenApiCredentialSecretResp{Base: okBase(), Data: &core.OpenApiCredentialSecret{
		Credential: mapOpenApiCredential(item), ApiKey: fullKey,
	}}, nil
}

func listOpenApiCredentials(ctx context.Context, svcCtx *svc.ServiceContext, in *core.OpenApiCredentialListReq) (*core.OpenApiCredentialListResp, error) {
	if in == nil {
		in = &core.OpenApiCredentialListReq{}
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.Page)
	where := []string{"tenant_id=?"}
	args := []any{tenant}
	if in.Status != core.OpenApiCredentialStatus_OPEN_API_CREDENTIAL_STATUS_UNKNOWN {
		where = append(where, "status=?")
		args = append(args, int64(in.Status))
	}
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		where = append(where, "(credential_name LIKE ? OR key_id LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(*) FROM t_open_api_credential WHERE "+strings.Join(where, " AND "), args...); err != nil {
		return nil, status.Errorf(codes.Internal, "count API credentials failed: %v", err)
	}
	where = append(where, "id>?")
	args = append(args, cursor)
	var items []models.TOpenApiCredential
	queryArgs := append(args, limit+1)
	if err := svcCtx.DB.QueryRowsCtx(ctx, &items, openApiCredentialSelect+" WHERE "+strings.Join(where, " AND ")+" ORDER BY id ASC LIMIT ?", queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list API credentials failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.OpenApiCredential, 0, len(items))
	var next int64
	for index := range items {
		data = append(data, mapOpenApiCredential(&items[index]))
		next = items[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.OpenApiCredentialListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

func rotateOpenApiCredential(ctx context.Context, svcCtx *svc.ServiceContext, in *core.RotateOpenApiCredentialReq) (*core.OpenApiCredentialSecretResp, error) {
	if in == nil || in.Id <= 0 || in.GraceSeconds < 0 || in.GraceSeconds > 30*24*60*60 {
		return nil, status.Error(codes.InvalidArgument, "credential id or grace_seconds is invalid")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	keyID, _, fullKey, secretHash, err := generateOpenApiKey()
	if err != nil {
		return nil, status.Error(codes.Internal, "generate API credential failed")
	}
	var created models.TOpenApiCredential
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var source models.TOpenApiCredential
		if err := session.QueryRowCtx(txCtx, &source, openApiCredentialSelect+" WHERE id=? AND tenant_id=? FOR UPDATE", in.Id, tenant); err != nil {
			return err
		}
		if source.Status != int64(core.OpenApiCredentialStatus_OPEN_API_CREDENTIAL_STATUS_ACTIVE) {
			return status.Error(codes.FailedPrecondition, "only active credential can be rotated")
		}
		result, err := session.ExecCtx(txCtx, `INSERT INTO t_open_api_credential
(tenant_id,credential_name,key_id,secret_hash,scopes,app_ids,ip_allowlist,rate_limit_per_minute,status,expires_at,rotated_from_id,create_by)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, source.TenantId, source.CredentialName, keyID, secretHash, source.Scopes,
			source.AppIds, source.IpAllowlist, source.RateLimitPerMinute,
			int64(core.OpenApiCredentialStatus_OPEN_API_CREDENTIAL_STATUS_ACTIVE), source.ExpiresAt, source.Id, actorID(ctx))
		if err != nil {
			return err
		}
		created.Id, _ = result.LastInsertId()
		oldStatus := int64(core.OpenApiCredentialStatus_OPEN_API_CREDENTIAL_STATUS_REVOKED)
		var grace any
		if in.GraceSeconds > 0 {
			oldStatus = int64(core.OpenApiCredentialStatus_OPEN_API_CREDENTIAL_STATUS_ROTATION_GRACE)
			grace = time.Now().Add(time.Duration(in.GraceSeconds) * time.Second)
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_open_api_credential SET status=?,grace_expires_at=?,update_time=CURRENT_TIMESTAMP(3) WHERE id=?`, oldStatus, grace, source.Id); err != nil {
			return err
		}
		return session.QueryRowCtx(txCtx, &created, openApiCredentialSelect+" WHERE id=?", created.Id)
	})
	if err != nil {
		if status.Code(err) != codes.Unknown {
			return nil, err
		}
		return nil, status.Errorf(codes.Internal, "rotate API credential failed: %v", err)
	}
	return &core.OpenApiCredentialSecretResp{Base: okBase(), Data: &core.OpenApiCredentialSecret{
		Credential: mapOpenApiCredential(&created), ApiKey: fullKey,
	}}, nil
}

func revokeOpenApiCredential(ctx context.Context, svcCtx *svc.ServiceContext, in *core.OpenApiCredentialIdReq) (*core.OpenApiCredentialResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "credential id is required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	var item models.TOpenApiCredential
	if err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := session.QueryRowCtx(txCtx, &item, openApiCredentialSelect+" WHERE id=? AND tenant_id=? FOR UPDATE", in.Id, tenant); err != nil {
			return err
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_open_api_credential SET status=?,grace_expires_at=NULL,update_time=CURRENT_TIMESTAMP(3) WHERE id=?`, int64(core.OpenApiCredentialStatus_OPEN_API_CREDENTIAL_STATUS_REVOKED), item.Id); err != nil {
			return err
		}
		item.Status = int64(core.OpenApiCredentialStatus_OPEN_API_CREDENTIAL_STATUS_REVOKED)
		item.GraceExpiresAt = sql.NullTime{}
		return nil
	}); err != nil {
		return nil, notFoundOrInternal(err, "API credential")
	}
	return &core.OpenApiCredentialResp{Base: okBase(), Data: mapOpenApiCredential(&item)}, nil
}

func authenticateOpenApiCredential(ctx context.Context, svcCtx *svc.ServiceContext, in *core.AuthenticateOpenApiCredentialReq) (*core.OpenApiAuthContextResp, error) {
	if in == nil || strings.TrimSpace(in.KeyId) == "" || !validSHA256(strings.TrimSpace(in.SecretHash)) {
		return nil, status.Error(codes.Unauthenticated, "invalid API credential")
	}
	var item models.TOpenApiCredential
	if err := svcCtx.DB.QueryRowCtx(ctx, &item, openApiCredentialSelect+" WHERE key_id=?", strings.TrimSpace(in.KeyId)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid API credential")
	}
	if subtle.ConstantTimeCompare([]byte(item.SecretHash), []byte(strings.TrimSpace(in.SecretHash))) != 1 {
		return nil, status.Error(codes.Unauthenticated, "invalid API credential")
	}
	now := time.Now()
	active := item.Status == int64(core.OpenApiCredentialStatus_OPEN_API_CREDENTIAL_STATUS_ACTIVE)
	grace := item.Status == int64(core.OpenApiCredentialStatus_OPEN_API_CREDENTIAL_STATUS_ROTATION_GRACE) && item.GraceExpiresAt.Valid && item.GraceExpiresAt.Time.After(now)
	if !active && !grace {
		return nil, status.Error(codes.Unauthenticated, "API credential is inactive")
	}
	if item.ExpiresAt.Valid && !item.ExpiresAt.Time.After(now) {
		_, _ = svcCtx.DB.ExecCtx(ctx, `UPDATE t_open_api_credential SET status=?,update_time=CURRENT_TIMESTAMP(3) WHERE id=?`, int64(core.OpenApiCredentialStatus_OPEN_API_CREDENTIAL_STATUS_EXPIRED), item.Id)
		return nil, status.Error(codes.Unauthenticated, "API credential has expired")
	}
	if !clientIPAllowed(in.ClientIp, parseStringJSON(item.IpAllowlist)) {
		return nil, status.Error(codes.PermissionDenied, "client IP is not allowed")
	}
	subscription, entitlement, _, err := currentBilling(svcCtx, ctx, item.TenantId)
	if err != nil || !subscriptionAllowsConsumption(subscription, billingNow()) ||
		entitlement.Status != entitlementActive || !billingNow().Before(entitlement.ValidUntil) {
		return nil, status.Error(codes.PermissionDenied, "tenant subscription does not allow Open API access")
	}
	effectiveRateLimit := item.RateLimitPerMinute
	if entitlement.ApiRateLimit == 0 {
		return nil, status.Error(codes.ResourceExhausted, "Open API is disabled by the current entitlement")
	}
	if entitlement.ApiRateLimit > 0 && entitlement.ApiRateLimit < effectiveRateLimit {
		effectiveRateLimit = entitlement.ApiRateLimit
	}
	_, _ = svcCtx.DB.ExecCtx(ctx, `UPDATE t_open_api_credential SET last_used_at=CURRENT_TIMESTAMP(3),update_time=CURRENT_TIMESTAMP(3) WHERE id=?`, item.Id)
	return &core.OpenApiAuthContextResp{Base: okBase(), Data: &core.OpenApiAuthContext{
		CredentialId: item.Id, TenantId: item.TenantId, KeyId: item.KeyId,
		Scopes: parseOpenApiScopes(item.Scopes), AppIds: parseInt64JSON(item.AppIds),
		RateLimitPerMinute: int32(effectiveRateLimit),
	}}, nil
}

func clientIPAllowed(value string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	for _, allowed := range allowlist {
		if exact, err := netip.ParseAddr(allowed); err == nil && exact == addr {
			return true
		}
		if prefix, err := netip.ParsePrefix(allowed); err == nil && prefix.Contains(addr) {
			return true
		}
	}
	return false
}
