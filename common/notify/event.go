package notify

import (
	"context"
	"fmt"
	"time"

	"appforge/common/mq/kafka"
)

const (
	Channel = "admin:notifications"

	EventTypeUserIdentitySubmit = "user_identity_submit"
	EventTypeRecharge           = "recharge"
	EventTypeWithdraw           = "withdraw"

	EventLevelInfo    = "info"
	EventLevelWarning = "warning"
	EventLevelError   = "error"
)

type Event struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Level     string         `json:"level"`
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	Source    string         `json:"source,omitempty"`
	TenantID  int64          `json:"tenantId,omitempty"`
	UserID    int64          `json:"userId,omitempty"`
	BizNo     string         `json:"bizNo,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	CreatedAt int64          `json:"createdAt"`
}

func NewEvent(eventType, level, title, message string) Event {
	now := time.Now().UnixMilli()
	if level == "" {
		level = EventLevelInfo
	}

	return Event{
		ID:        fmt.Sprintf("%d", now),
		Type:      eventType,
		Level:     level,
		Title:     title,
		Message:   message,
		CreatedAt: now,
	}
}

func Publish(ctx context.Context, publisher *mq.Publisher, event Event) error {
	return publisher.Publish(ctx, Channel, event)
}
