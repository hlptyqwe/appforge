package userevent

import (
	"context"
	"fmt"
	"strconv"
	"time"

	mq "appforge/common/mq/kafka"
)

const (
	Channel          = "user:business-events"
	VersionV1        = int64(1)
	TypeOrderChanged = "order.changed"

	DomainTrade   = "trade"
	DomainOption  = "option"
	DomainStaking = "staking"
)

type Event struct {
	Version     int64  `json:"version"`
	ID          string `json:"id"`
	Type        string `json:"type"`
	Domain      string `json:"domain"`
	TenantID    int64  `json:"tenant_id"`
	UserID      int64  `json:"user_id"`
	BizID       int64  `json:"biz_id,omitempty"`
	BizNo       string `json:"biz_no,omitempty"`
	SymbolID    int64  `json:"symbol_id,omitempty"`
	ProductType int64  `json:"product_type,omitempty"`
	ChangeType  string `json:"change_type,omitempty"`
	OccurredAt  int64  `json:"occurred_at"`
}

func NewOrderChanged(domain string, tenantID, userID, bizID int64, bizNo string) Event {
	now := time.Now().UnixMilli()
	return Event{
		Version:    VersionV1,
		ID:         fmt.Sprintf("%s:%s:%d", domain, bizNo, now),
		Type:       TypeOrderChanged,
		Domain:     domain,
		TenantID:   tenantID,
		UserID:     userID,
		BizID:      bizID,
		BizNo:      bizNo,
		OccurredAt: now,
	}
}

func Publish(ctx context.Context, publisher *mq.Publisher, event Event) error {
	key := []byte(strconv.FormatInt(event.TenantID, 10) + ":" + strconv.FormatInt(event.UserID, 10))
	return publisher.PublishKey(ctx, Channel, key, event)
}
