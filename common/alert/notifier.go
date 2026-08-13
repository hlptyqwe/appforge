package alert

import (
	"context"
	"errors"
	"fmt"
)

var ErrNotifierRequired = errors.New("alert notifier is required")

// Notifier is the transport-independent alert delivery contract.
//
// Implementations may deliver to Kafka, WebSocket, webhook, email, SMS or any
// other notification channel. Implementations must not mutate the Alert.
type Notifier interface {
	Notify(context.Context, Alert) error
}

// NotifierFunc lets a function implement Notifier.
type NotifierFunc func(context.Context, Alert) error

func (f NotifierFunc) Notify(ctx context.Context, value Alert) error {
	if f == nil {
		return ErrNotifierRequired
	}
	return f(ctx, value)
}

// Notify validates a domain alert before handing it to a delivery
// implementation. Application code should use this function rather than
// depending on a concrete transport.
func Notify(ctx context.Context, notifier Notifier, value Alert) error {
	if notifier == nil {
		return ErrNotifierRequired
	}
	if err := value.Validate(); err != nil {
		return err
	}
	return notifier.Notify(ctx, value)
}

// MultiNotifier delivers an alert to every configured notifier. All notifiers
// are attempted even if one fails; the returned error joins every failure.
// Alert.ID is the idempotency key implementations should use when retrying.
type MultiNotifier struct {
	notifiers []Notifier
}

func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	return &MultiNotifier{notifiers: append([]Notifier(nil), notifiers...)}
}

func (m *MultiNotifier) Notify(ctx context.Context, value Alert) error {
	if m == nil || len(m.notifiers) == 0 {
		return ErrNotifierRequired
	}
	var errs []error
	for index, notifier := range m.notifiers {
		if err := Notify(ctx, notifier, value); err != nil {
			errs = append(errs, fmt.Errorf("alert notifier %d: %w", index, err))
		}
	}
	return errors.Join(errs...)
}
