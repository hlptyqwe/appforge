package logic

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var supportedWebhookEvents = map[string]struct{}{
	"build.queued":      {},
	"build.started":     {},
	"build.succeeded":   {},
	"build.failed":      {},
	"build.canceled":    {},
	"artifact.expiring": {},
	"quota.warning":     {},
	"quota.exceeded":    {},
}

func normalizeWebhookEvents(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if _, ok := supportedWebhookEvents[value]; !ok {
			return nil, status.Errorf(codes.InvalidArgument, "unsupported webhook event type: %s", value)
		}
		seen[value] = struct{}{}
	}
	if len(seen) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one webhook event type is required")
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func validateWebhookEndpointURL(ctx context.Context, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return status.Error(codes.InvalidArgument, "webhook URL must be an HTTPS URL without userinfo")
	}
	if len(raw) > 1000 || parsed.Fragment != "" {
		return status.Error(codes.InvalidArgument, "webhook URL is invalid or too long")
	}
	if strings.EqualFold(parsed.Hostname(), "metadata.google.internal") || strings.HasSuffix(strings.ToLower(parsed.Hostname()), ".localhost") {
		return status.Error(codes.InvalidArgument, "webhook URL cannot target a local or metadata host")
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", parsed.Hostname())
	if err != nil || len(ips) == 0 {
		return status.Error(codes.InvalidArgument, "webhook hostname cannot be resolved")
	}
	for _, ip := range ips {
		if isForbiddenWebhookIP(ip) {
			return status.Error(codes.InvalidArgument, "webhook URL resolves to a private or reserved address")
		}
	}
	return nil
}

func isForbiddenWebhookIP(ip net.IP) bool {
	return ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast()
}

func createWebhookEndpoint(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CreateWebhookEndpointReq) (*core.WebhookEndpointSecretResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireText(in.EndpointName, "endpoint_name", 128); err != nil {
		return nil, err
	}
	validateCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := validateWebhookEndpointURL(validateCtx, in.EndpointUrl); err != nil {
		return nil, err
	}
	events, err := normalizeWebhookEvents(in.EventTypes)
	if err != nil {
		return nil, err
	}
	maxAttempts := in.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 8
	}
	if maxAttempts < 1 || maxAttempts > 20 {
		return nil, status.Error(codes.InvalidArgument, "max_attempts must be between 1 and 20")
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, status.Errorf(codes.Internal, "generate webhook secret failed: %v", err)
	}
	secret := "whsec_" + base64.RawURLEncoding.EncodeToString(secretBytes)
	ciphertext, err := svcCtx.Secrets.Seal(secret)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "encrypt webhook secret failed: %v", err)
	}
	eventJSON, _ := json.Marshal(events)
	result, err := svcCtx.WebhookEndpointModel.Insert(ctx, &models.TWebhookEndpoint{
		TenantId: tenant, EndpointName: strings.TrimSpace(in.EndpointName), EndpointUrl: strings.TrimSpace(in.EndpointUrl),
		EventTypes: string(eventJSON), SecretCiphertext: ciphertext, SecretHint: secret[len(secret)-6:],
		MaxAttempts: int64(maxAttempts), Status: int64(core.WebhookEndpointStatus_WEBHOOK_ENDPOINT_STATUS_ACTIVE), CreateBy: actorID(ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create webhook endpoint failed: %v", err)
	}
	id, _ := result.LastInsertId()
	item, err := svcCtx.WebhookEndpointModel.FindOne(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load created webhook endpoint failed: %v", err)
	}
	return &core.WebhookEndpointSecretResp{Base: okBase(), Data: &core.WebhookEndpointSecret{Endpoint: mapWebhookEndpoint(item), SigningSecret: secret}}, nil
}

func updateWebhookEndpoint(ctx context.Context, svcCtx *svc.ServiceContext, in *core.UpdateWebhookEndpointReq) (*core.WebhookEndpointResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := svcCtx.WebhookEndpointModel.FindOne(ctx, in.Id)
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "webhook endpoint not found")
	}
	if err := requireText(in.EndpointName, "endpoint_name", 128); err != nil {
		return nil, err
	}
	validateCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := validateWebhookEndpointURL(validateCtx, in.EndpointUrl); err != nil {
		return nil, err
	}
	events, err := normalizeWebhookEvents(in.EventTypes)
	if err != nil {
		return nil, err
	}
	if in.MaxAttempts < 1 || in.MaxAttempts > 20 {
		return nil, status.Error(codes.InvalidArgument, "max_attempts must be between 1 and 20")
	}
	if in.Status < core.WebhookEndpointStatus_WEBHOOK_ENDPOINT_STATUS_ACTIVE || in.Status > core.WebhookEndpointStatus_WEBHOOK_ENDPOINT_STATUS_REVOKED {
		return nil, status.Error(codes.InvalidArgument, "invalid webhook endpoint status")
	}
	eventJSON, _ := json.Marshal(events)
	item.EndpointName = strings.TrimSpace(in.EndpointName)
	item.EndpointUrl = strings.TrimSpace(in.EndpointUrl)
	item.EventTypes = string(eventJSON)
	item.MaxAttempts = int64(in.MaxAttempts)
	item.Status = int64(in.Status)
	if err := svcCtx.WebhookEndpointModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update webhook endpoint failed: %v", err)
	}
	return &core.WebhookEndpointResp{Base: okBase(), Data: mapWebhookEndpoint(item)}, nil
}

func getWebhookEndpoint(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WebhookEndpointIdReq) (*core.WebhookEndpointResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := svcCtx.WebhookEndpointModel.FindOne(ctx, in.Id)
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "webhook endpoint not found")
	}
	return &core.WebhookEndpointResp{Base: okBase(), Data: mapWebhookEndpoint(item)}, nil
}

func listWebhookEndpoints(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WebhookEndpointListReq) (*core.WebhookEndpointListResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.GetPage())
	where := []string{"tenant_id = ?"}
	args := []any{tenant}
	if in.GetStatus() != core.WebhookEndpointStatus_WEBHOOK_ENDPOINT_STATUS_UNKNOWN {
		where = append(where, "status = ?")
		args = append(args, int64(in.GetStatus()))
	}
	if keyword := strings.TrimSpace(in.GetKeyword()); keyword != "" {
		where = append(where, "(endpoint_name LIKE ? OR endpoint_url LIKE ?)")
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM t_webhook_endpoint WHERE "+whereSQL, args...); err != nil {
		return nil, status.Errorf(codes.Internal, "count webhook endpoints failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var rows []models.TWebhookEndpoint
	query := `SELECT id, tenant_id, endpoint_name, endpoint_url, event_types, secret_ciphertext, secret_hint,
max_attempts, status, last_success_at, last_failure_at, create_by, create_time, update_time
FROM t_webhook_endpoint WHERE ` + whereSQL + ` AND id > ? ORDER BY id ASC LIMIT ?`
	if err := svcCtx.DB.QueryRowsCtx(ctx, &rows, query, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list webhook endpoints failed: %v", err)
	}
	hasNext := int64(len(rows)) > limit
	if hasNext {
		rows = rows[:limit]
	}
	data := make([]*core.WebhookEndpoint, 0, len(rows))
	var next int64
	for index := range rows {
		data = append(data, mapWebhookEndpoint(&rows[index]))
		next = rows[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.WebhookEndpointListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

func rotateWebhookEndpointSecret(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WebhookEndpointIdReq) (*core.WebhookEndpointSecretResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := svcCtx.WebhookEndpointModel.FindOne(ctx, in.GetId())
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "webhook endpoint not found")
	}
	if item.Status == int64(core.WebhookEndpointStatus_WEBHOOK_ENDPOINT_STATUS_REVOKED) {
		return nil, status.Error(codes.FailedPrecondition, "revoked webhook endpoint cannot rotate its secret")
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, status.Errorf(codes.Internal, "generate webhook secret failed: %v", err)
	}
	secret := "whsec_" + base64.RawURLEncoding.EncodeToString(secretBytes)
	item.SecretCiphertext, err = svcCtx.Secrets.Seal(secret)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "encrypt webhook secret failed: %v", err)
	}
	item.SecretHint = secret[len(secret)-6:]
	if err := svcCtx.WebhookEndpointModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "rotate webhook secret failed: %v", err)
	}
	return &core.WebhookEndpointSecretResp{Base: okBase(), Data: &core.WebhookEndpointSecret{Endpoint: mapWebhookEndpoint(item), SigningSecret: secret}}, nil
}

func listWebhookDeliveries(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WebhookDeliveryListReq) (*core.WebhookDeliveryListResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.GetPage())
	where := []string{"d.tenant_id = ?"}
	args := []any{tenant}
	if in.GetEndpointId() > 0 {
		where = append(where, "d.endpoint_id = ?")
		args = append(args, in.GetEndpointId())
	}
	if in.GetStatus() != core.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_UNKNOWN {
		where = append(where, "d.status = ?")
		args = append(args, int64(in.GetStatus()))
	}
	if eventType := strings.TrimSpace(in.GetEventType()); eventType != "" {
		where = append(where, "o.event_type = ?")
		args = append(args, eventType)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, `SELECT COUNT(1) FROM t_webhook_delivery d
JOIN t_outbox_event o ON o.id = d.outbox_event_id WHERE `+whereSQL, args...); err != nil {
		return nil, status.Errorf(codes.Internal, "count webhook deliveries failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var rows []webhookDeliveryRow
	if err := svcCtx.DB.QueryRowsCtx(ctx, &rows, webhookDeliverySelect+` WHERE `+whereSQL+` AND d.id > ? ORDER BY d.id ASC LIMIT ?`, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list webhook deliveries failed: %v", err)
	}
	hasNext := int64(len(rows)) > limit
	if hasNext {
		rows = rows[:limit]
	}
	data := make([]*core.WebhookDelivery, 0, len(rows))
	var next int64
	for index := range rows {
		data = append(data, mapWebhookDelivery(&rows[index]))
		next = rows[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.WebhookDeliveryListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

func replayWebhookDelivery(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WebhookDeliveryIdReq) (*core.WebhookDeliveryResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	result, err := svcCtx.DB.ExecCtx(ctx, `UPDATE t_webhook_delivery SET attempt = 0, status = 1,
response_status = 0, response_body_excerpt = NULL, error_message = NULL, next_retry_at = CURRENT_TIMESTAMP(3),
lease_until = NULL, delivered_at = NULL, update_time = CURRENT_TIMESTAMP(3)
WHERE id = ? AND tenant_id = ? AND status IN (3,4,5)`, in.Id, tenant)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "replay webhook delivery failed: %v", err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, status.Error(codes.FailedPrecondition, "webhook delivery cannot be replayed")
	}
	var row webhookDeliveryRow
	if err := svcCtx.DB.QueryRowCtx(ctx, &row, webhookDeliverySelect+` WHERE d.id = ? AND d.tenant_id = ?`, in.Id, tenant); err != nil {
		return nil, status.Errorf(codes.Internal, "load replayed webhook delivery failed: %v", err)
	}
	return &core.WebhookDeliveryResp{Base: okBase(), Data: mapWebhookDelivery(&row)}, nil
}

func createTestWebhookEvent(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CreateTestWebhookEventReq) (*core.RespBase, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.EndpointId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "endpoint_id is required")
	}
	endpoint, err := svcCtx.WebhookEndpointModel.FindOne(ctx, in.EndpointId)
	if err != nil || endpoint.TenantId != tenant || endpoint.Status != int64(core.WebhookEndpointStatus_WEBHOOK_ENDPOINT_STATUS_ACTIVE) {
		return nil, status.Error(codes.NotFound, "active webhook endpoint not found")
	}
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		outboxID, eventID, err := insertOutboxEvent(txCtx, session, tenant, "webhook.test", "webhook_endpoint", endpoint.Id,
			map[string]any{"endpointId": endpoint.Id, "message": "AppForge webhook test"})
		if err != nil {
			return err
		}
		if _, err := session.ExecCtx(txCtx, `INSERT INTO t_webhook_delivery
(tenant_id, endpoint_id, outbox_event_id, event_id, attempt, status, response_status, next_retry_at)
VALUES (?, ?, ?, ?, 0, 1, 0, CURRENT_TIMESTAMP(3))`, tenant, endpoint.Id, outboxID, eventID); err != nil {
			return err
		}
		_, err = session.ExecCtx(txCtx, `UPDATE t_outbox_event SET status = 2, dispatched_at = CURRENT_TIMESTAMP(3) WHERE id = ?`, outboxID)
		return err
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create test webhook event failed: %v", err)
	}
	return &core.RespBase{Base: okBase()}, nil
}

func mapWebhookEndpoint(item *models.TWebhookEndpoint) *core.WebhookEndpoint {
	if item == nil {
		return nil
	}
	var events []string
	_ = json.Unmarshal([]byte(item.EventTypes), &events)
	return &core.WebhookEndpoint{Id: item.Id, TenantId: item.TenantId, EndpointName: item.EndpointName,
		EndpointUrl: item.EndpointUrl, EventTypes: events, SecretHint: item.SecretHint, MaxAttempts: int32(item.MaxAttempts),
		Status: core.WebhookEndpointStatus(item.Status), LastSuccessAt: millis(item.LastSuccessAt.Time),
		LastFailureAt: millis(item.LastFailureAt.Time), CreateBy: item.CreateBy,
		CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime)}
}

type webhookDeliveryRow struct {
	Id                  int64          `db:"id"`
	TenantId            int64          `db:"tenant_id"`
	EndpointId          int64          `db:"endpoint_id"`
	EventId             string         `db:"event_id"`
	EventType           string         `db:"event_type"`
	Attempt             int64          `db:"attempt"`
	Status              int64          `db:"status"`
	ResponseStatus      int64          `db:"response_status"`
	ResponseBodyExcerpt sql.NullString `db:"response_body_excerpt"`
	ErrorMessage        sql.NullString `db:"error_message"`
	NextRetryAt         time.Time      `db:"next_retry_at"`
	DeliveredAt         sql.NullTime   `db:"delivered_at"`
	CreateTime          time.Time      `db:"create_time"`
	UpdateTime          time.Time      `db:"update_time"`
}

const webhookDeliverySelect = `SELECT d.id, d.tenant_id, d.endpoint_id, d.event_id, o.event_type,
d.attempt, d.status, d.response_status, d.response_body_excerpt, d.error_message,
d.next_retry_at, d.delivered_at, d.create_time, d.update_time
FROM t_webhook_delivery d JOIN t_outbox_event o ON o.id = d.outbox_event_id`

func mapWebhookDelivery(item *webhookDeliveryRow) *core.WebhookDelivery {
	if item == nil {
		return nil
	}
	return &core.WebhookDelivery{Id: item.Id, TenantId: item.TenantId, EndpointId: item.EndpointId,
		EventId: item.EventId, EventType: item.EventType, Attempt: int32(item.Attempt), Status: core.WebhookDeliveryStatus(item.Status),
		ResponseStatus: int32(item.ResponseStatus), ResponseBodyExcerpt: stringValue(item.ResponseBodyExcerpt),
		ErrorMessage: stringValue(item.ErrorMessage), NextRetryAt: millis(item.NextRetryAt),
		DeliveredAt: millis(item.DeliveredAt.Time), CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime)}
}

func insertOutboxEvent(ctx context.Context, session sqlx.Session, tenant int64, eventType, aggregateType string, aggregateID int64, payload any) (int64, string, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, "", err
	}
	eventID, err := newWebhookEventID()
	if err != nil {
		return 0, "", err
	}
	result, err := session.ExecCtx(ctx, `INSERT INTO t_outbox_event
(tenant_id, event_id, event_type, aggregate_type, aggregate_id, schema_version, payload, status, occurred_at)
VALUES (?, ?, ?, ?, ?, 1, CAST(? AS JSON), 1, CURRENT_TIMESTAMP(3))`, tenant, eventID, eventType, aggregateType, aggregateID, string(payloadJSON))
	if err != nil {
		return 0, "", err
	}
	id, err := result.LastInsertId()
	return id, eventID, err
}

func newWebhookEventID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value)
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}
