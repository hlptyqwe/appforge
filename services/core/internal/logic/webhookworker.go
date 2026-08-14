package logic

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type WebhookWorker struct {
	svcCtx       *svc.ServiceContext
	pollInterval time.Duration
	httpTimeout  time.Duration
	batchSize    int
	allowPrivate bool
}

func NewWebhookWorker(svcCtx *svc.ServiceContext) *WebhookWorker {
	pollInterval := svcCtx.Config.WebhookWorker.PollInterval
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	httpTimeout := svcCtx.Config.WebhookWorker.HttpTimeout
	if httpTimeout <= 0 {
		httpTimeout = 10 * time.Second
	}
	batchSize := int(svcCtx.Config.WebhookWorker.BatchSize)
	if batchSize <= 0 || batchSize > 100 {
		batchSize = 20
	}
	return &WebhookWorker{svcCtx: svcCtx, pollInterval: pollInterval, httpTimeout: httpTimeout, batchSize: batchSize}
}

func (w *WebhookWorker) Start(ctx context.Context) {
	if w == nil || w.svcCtx == nil || !w.svcCtx.Config.WebhookWorker.Enabled {
		return
	}
	go func() {
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()
		w.runCycle(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.runCycle(ctx)
			}
		}
	}()
}

func (w *WebhookWorker) runCycle(ctx context.Context) {
	if err := w.dispatchOutbox(ctx); err != nil {
		logx.WithContext(ctx).Errorf("dispatch webhook outbox failed: %v", err)
		return
	}
	for index := 0; index < w.batchSize; index++ {
		delivery, err := w.claimDelivery(ctx)
		if err != nil {
			logx.WithContext(ctx).Errorf("claim webhook delivery failed: %v", err)
			return
		}
		if delivery == nil {
			return
		}
		w.deliver(ctx, delivery)
	}
}

type outboxDispatchRow struct {
	Id        int64  `db:"id"`
	TenantId  int64  `db:"tenant_id"`
	EventId   string `db:"event_id"`
	EventType string `db:"event_type"`
}

func (w *WebhookWorker) dispatchOutbox(ctx context.Context) error {
	var pending []outboxDispatchRow
	if err := w.svcCtx.DB.QueryRowsCtx(ctx, &pending, `SELECT id, tenant_id, event_id, event_type
FROM t_outbox_event WHERE status = 1 ORDER BY occurred_at ASC, id ASC LIMIT ?`, w.batchSize); err != nil {
		return err
	}
	for _, candidate := range pending {
		err := w.svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
			var event outboxDispatchRow
			if err := session.QueryRowCtx(txCtx, &event, `SELECT id, tenant_id, event_id, event_type
FROM t_outbox_event WHERE id = ? AND status = 1 FOR UPDATE`, candidate.Id); err != nil {
				return nil
			}
			var endpoints []struct {
				Id         int64  `db:"id"`
				EventTypes string `db:"event_types"`
			}
			if err := session.QueryRowsCtx(txCtx, &endpoints, `SELECT id, event_types FROM t_webhook_endpoint
WHERE tenant_id = ? AND status = 1 ORDER BY id`, event.TenantId); err != nil {
				return err
			}
			created := 0
			for _, endpoint := range endpoints {
				var subscribed []string
				if json.Unmarshal([]byte(endpoint.EventTypes), &subscribed) != nil || !containsString(subscribed, event.EventType) {
					continue
				}
				result, err := session.ExecCtx(txCtx, `INSERT IGNORE INTO t_webhook_delivery
(tenant_id, endpoint_id, outbox_event_id, event_id, attempt, status, response_status, next_retry_at)
VALUES (?, ?, ?, ?, 0, 1, 0, CURRENT_TIMESTAMP(3))`, event.TenantId, endpoint.Id, event.Id, event.EventId)
				if err != nil {
					return err
				}
				if affected, _ := result.RowsAffected(); affected == 1 {
					created++
				}
			}
			statusValue := 2
			if created == 0 {
				statusValue = 3
			}
			_, err := session.ExecCtx(txCtx, `UPDATE t_outbox_event SET status = ?, dispatched_at = CURRENT_TIMESTAMP(3),
update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, statusValue, event.Id)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type webhookClaim struct {
	DeliveryId       int64     `db:"delivery_id"`
	TenantId         int64     `db:"tenant_id"`
	EndpointId       int64     `db:"endpoint_id"`
	EndpointUrl      string    `db:"endpoint_url"`
	SecretCiphertext string    `db:"secret_ciphertext"`
	MaxAttempts      int64     `db:"max_attempts"`
	OutboxEventId    int64     `db:"outbox_event_id"`
	EventId          string    `db:"event_id"`
	EventType        string    `db:"event_type"`
	AggregateType    string    `db:"aggregate_type"`
	AggregateId      int64     `db:"aggregate_id"`
	SchemaVersion    int64     `db:"schema_version"`
	Payload          string    `db:"payload"`
	OccurredAt       time.Time `db:"occurred_at"`
	Attempt          int64     `db:"attempt"`
}

func (w *WebhookWorker) claimDelivery(ctx context.Context) (*webhookClaim, error) {
	var claimed *webhookClaim
	err := w.svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var row webhookClaim
		err := session.QueryRowCtx(txCtx, &row, `SELECT d.id AS delivery_id, d.tenant_id, d.endpoint_id,
e.endpoint_url, e.secret_ciphertext, e.max_attempts, d.outbox_event_id, d.event_id,
o.event_type, o.aggregate_type, o.aggregate_id, o.schema_version, o.payload, o.occurred_at, d.attempt
FROM t_webhook_delivery d
JOIN t_webhook_endpoint e ON e.id = d.endpoint_id AND e.tenant_id = d.tenant_id AND e.status = 1
JOIN t_outbox_event o ON o.id = d.outbox_event_id
WHERE ((d.status IN (1,4) AND d.next_retry_at <= CURRENT_TIMESTAMP(3))
OR (d.status = 2 AND d.lease_until <= CURRENT_TIMESTAMP(3)))
ORDER BY d.next_retry_at ASC, d.id ASC LIMIT 1 FOR UPDATE SKIP LOCKED`)
		if err != nil {
			if errors.Is(err, sqlx.ErrNotFound) {
				return nil
			}
			return err
		}
		result, err := session.ExecCtx(txCtx, `UPDATE t_webhook_delivery SET status = 2, attempt = attempt + 1,
lease_until = DATE_ADD(CURRENT_TIMESTAMP(3), INTERVAL 30 SECOND), update_time = CURRENT_TIMESTAMP(3)
WHERE id = ? AND attempt = ?`, row.DeliveryId, row.Attempt)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return nil
		}
		row.Attempt++
		claimed = &row
		return nil
	})
	return claimed, err
}

type webhookEnvelope struct {
	EventID       string          `json:"eventId"`
	EventType     string          `json:"eventType"`
	OccurredAt    int64           `json:"occurredAt"`
	TenantID      int64           `json:"tenantId"`
	Data          json.RawMessage `json:"data"`
	SchemaVersion int64           `json:"schemaVersion"`
}

func (w *WebhookWorker) deliver(ctx context.Context, delivery *webhookClaim) {
	secret, err := w.svcCtx.Secrets.Open(delivery.SecretCiphertext)
	if err != nil {
		w.finishDelivery(ctx, delivery, 0, "", "decrypt webhook secret failed")
		return
	}
	body, err := json.Marshal(webhookEnvelope{EventID: delivery.EventId, EventType: delivery.EventType,
		OccurredAt: delivery.OccurredAt.UnixMilli(), TenantID: delivery.TenantId,
		Data: json.RawMessage(delivery.Payload), SchemaVersion: delivery.SchemaVersion})
	if err != nil {
		w.finishDelivery(ctx, delivery, 0, "", "encode webhook payload failed")
		return
	}
	responseStatus, excerpt, err := sendWebhook(ctx, delivery.EndpointUrl, delivery.EventId, secret, body, w.httpTimeout, w.allowPrivate)
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}
	w.finishDelivery(ctx, delivery, responseStatus, excerpt, errorMessage)
}

func sendWebhook(ctx context.Context, endpointURL, eventID, secret string, body []byte, timeout time.Duration, allowPrivate bool) (int, string, error) {
	parsed, err := url.Parse(endpointURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return 0, "", errors.New("invalid webhook endpoint URL")
	}
	transport := &http.Transport{Proxy: nil, DialContext: webhookDialContext(allowPrivate)}
	client := &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return errors.New("webhook redirects are disabled")
	}}
	return sendWebhookWithClient(ctx, client, endpointURL, eventID, secret, body)
}

func sendWebhookWithClient(ctx context.Context, client *http.Client, endpointURL, eventID, secret string, body []byte) (int, string, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "AppForge-Webhook/1.0")
	request.Header.Set("X-AppForge-Event-Id", eventID)
	request.Header.Set("X-AppForge-Timestamp", timestamp)
	request.Header.Set("X-AppForge-Signature", webhookSignature(secret, timestamp, body))
	response, err := client.Do(request)
	if err != nil {
		return 0, "", err
	}
	defer response.Body.Close()
	excerptBytes, readErr := io.ReadAll(io.LimitReader(response.Body, 1000))
	excerpt := strings.TrimSpace(string(excerptBytes))
	if readErr != nil {
		return response.StatusCode, excerpt, readErr
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, excerpt, fmt.Errorf("webhook returned HTTP %d", response.StatusCode)
	}
	return response.StatusCode, excerpt, nil
}

func webhookDialContext(allowPrivate bool) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil || len(ips) == 0 {
			return nil, errors.New("webhook hostname cannot be resolved")
		}
		dialer := &net.Dialer{Timeout: 5 * time.Second}
		var lastErr error
		for _, ip := range ips {
			if !allowPrivate && isForbiddenWebhookIP(ip) {
				return nil, errors.New("webhook destination resolved to a private or reserved address")
			}
			connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return connection, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}

func webhookSignature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

func (w *WebhookWorker) finishDelivery(ctx context.Context, delivery *webhookClaim, responseStatus int, excerpt, errorMessage string) {
	succeeded := errorMessage == "" && responseStatus >= 200 && responseStatus < 300
	statusValue := 3
	nextRetrySeconds := int64(0)
	if !succeeded {
		statusValue = 4
		if delivery.Attempt >= delivery.MaxAttempts {
			statusValue = 5
		} else {
			nextRetrySeconds = webhookRetryDelay(delivery.Attempt)
		}
	}
	if len(excerpt) > 1000 {
		excerpt = excerpt[:1000]
	}
	if len(errorMessage) > 1000 {
		errorMessage = errorMessage[:1000]
	}
	err := w.svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if succeeded {
			if _, err := session.ExecCtx(txCtx, `UPDATE t_webhook_delivery SET status = 3, response_status = ?,
response_body_excerpt = NULLIF(?, ''), error_message = NULL, lease_until = NULL, delivered_at = CURRENT_TIMESTAMP(3),
update_time = CURRENT_TIMESTAMP(3) WHERE id = ? AND status = 2 AND attempt = ?`, responseStatus, excerpt, delivery.DeliveryId, delivery.Attempt); err != nil {
				return err
			}
			_, _ = session.ExecCtx(txCtx, `UPDATE t_webhook_endpoint SET last_success_at = CURRENT_TIMESTAMP(3),
update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, delivery.EndpointId)
		} else {
			if _, err := session.ExecCtx(txCtx, `UPDATE t_webhook_delivery SET status = ?, response_status = ?,
response_body_excerpt = NULLIF(?, ''), error_message = NULLIF(?, ''), lease_until = NULL,
next_retry_at = DATE_ADD(CURRENT_TIMESTAMP(3), INTERVAL ? SECOND), update_time = CURRENT_TIMESTAMP(3)
WHERE id = ? AND status = 2 AND attempt = ?`, statusValue, responseStatus, excerpt, errorMessage,
				nextRetrySeconds, delivery.DeliveryId, delivery.Attempt); err != nil {
				return err
			}
			_, _ = session.ExecCtx(txCtx, `UPDATE t_webhook_endpoint SET last_failure_at = CURRENT_TIMESTAMP(3),
update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, delivery.EndpointId)
		}
		if statusValue == 3 || statusValue == 5 {
			_, _ = session.ExecCtx(txCtx, `UPDATE t_outbox_event o SET o.status = 3, o.update_time = CURRENT_TIMESTAMP(3)
WHERE o.id = ? AND NOT EXISTS (SELECT 1 FROM t_webhook_delivery d WHERE d.outbox_event_id = o.id AND d.status NOT IN (3,5))`, delivery.OutboxEventId)
		}
		return nil
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("finish webhook delivery failed: deliveryId=%d err=%v", delivery.DeliveryId, err)
	}
}

func webhookRetryDelay(attempt int64) int64 {
	exponent := min(max(attempt, 1), 10)
	return int64(1) << exponent
}
