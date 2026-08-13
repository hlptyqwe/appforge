package adminnotify

import (
	"context"
	"errors"

	"appforge/common/alert"
	"appforge/common/notify"
)

// MessagePublisher is the minimal channel-based contract used by the current
// Admin notification transport.
type MessagePublisher interface {
	Publish(context.Context, string, any) error
}

// Notifier converts a domain alert into the existing Admin notification event.
type Notifier struct {
	publisher MessagePublisher
}

func New(publisher MessagePublisher) *Notifier {
	return &Notifier{publisher: publisher}
}

func (n *Notifier) Notify(ctx context.Context, value alert.Alert) error {
	if n == nil || n.publisher == nil {
		return errors.New("admin notification publisher is required")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	level := value.Severity
	if value.State == alert.StateResolved {
		level = notify.EventLevelInfo
	}
	event := notify.Event{
		ID:        value.ID,
		Type:      value.Type,
		Level:     level,
		Title:     value.Title,
		Message:   value.Message,
		Source:    value.Source,
		TenantID:  value.TenantID,
		BizNo:     value.Key,
		CreatedAt: value.CreatedAt,
		Data: map[string]any{
			"state":    value.State,
			"severity": value.Severity,
			"alertKey": value.Key,
			"details":  value.Data,
		},
	}
	return n.publisher.Publish(ctx, notify.Channel, event)
}

var _ alert.Notifier = (*Notifier)(nil)
