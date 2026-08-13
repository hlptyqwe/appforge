package alert

import (
	"context"
	"errors"
	"testing"
)

type capturedNotifier struct {
	value Alert
	err   error
}

func (n *capturedNotifier) Notify(_ context.Context, value Alert) error {
	n.value = value
	return n.err
}

func TestNotifyValidatesAndUsesInterface(t *testing.T) {
	value := New(
		TypeSnapshotOutbox,
		StateFiring,
		"error",
		"market",
		"snapshot-outbox",
		"Snapshot Outbox unhealthy",
		"pending rows exceeded the freshness window",
		1234,
	)
	notifier := &capturedNotifier{}
	if err := Notify(context.Background(), notifier, value); err != nil {
		t.Fatal(err)
	}
	if notifier.value.ID != value.ID || notifier.value.Key != value.Key {
		t.Fatalf("alert=%+v", notifier.value)
	}
}

func TestNotifyRejectsMissingNotifierAndInvalidAlert(t *testing.T) {
	if err := Notify(context.Background(), nil, Alert{}); !errors.Is(err, ErrNotifierRequired) {
		t.Fatalf("nil notifier error=%v", err)
	}
	value := New(TypeSnapshotOutbox, StateFiring, "error", "market", "key", "title", "message", 1234)
	value.State = "unknown"
	if err := Notify(context.Background(), &capturedNotifier{}, value); err == nil {
		t.Fatal("expected invalid state error")
	}
}

func TestNotifyReturnsImplementationError(t *testing.T) {
	notifierErr := errors.New("notification unavailable")
	value := New(TypeSnapshotOutbox, StateFiring, "error", "market", "key", "title", "message", 1234)
	if err := Notify(
		context.Background(),
		&capturedNotifier{err: notifierErr},
		value,
	); !errors.Is(err, notifierErr) {
		t.Fatalf("implementation error=%v", err)
	}
}

func TestMultiNotifierAttemptsEveryImplementation(t *testing.T) {
	value := New(TypeSnapshotOutbox, StateFiring, "error", "market", "key", "title", "message", 1234)
	var calls []string
	firstErr := errors.New("webhook unavailable")
	notifier := NewMultiNotifier(
		NotifierFunc(func(_ context.Context, got Alert) error {
			calls = append(calls, "webhook:"+got.ID)
			return firstErr
		}),
		NotifierFunc(func(_ context.Context, got Alert) error {
			calls = append(calls, "websocket:"+got.ID)
			return nil
		}),
	)
	err := Notify(context.Background(), notifier, value)
	if !errors.Is(err, firstErr) {
		t.Fatalf("joined error=%v", err)
	}
	if len(calls) != 2 ||
		calls[0] != "webhook:"+value.ID ||
		calls[1] != "websocket:"+value.ID {
		t.Fatalf("calls=%v", calls)
	}
}

func TestMultiNotifierRequiresAnImplementation(t *testing.T) {
	value := New(TypeSnapshotOutbox, StateFiring, "error", "market", "key", "title", "message", 1234)
	if err := Notify(
		context.Background(),
		NewMultiNotifier(),
		value,
	); !errors.Is(err, ErrNotifierRequired) {
		t.Fatalf("empty multi notifier error=%v", err)
	}
}
