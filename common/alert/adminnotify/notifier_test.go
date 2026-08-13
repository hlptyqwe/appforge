package adminnotify

import (
	"context"
	"errors"
	"testing"

	"appforge/common/alert"
	"appforge/common/notify"
)

type fakePublisher struct {
	channel string
	payload any
	err     error
}

func (f *fakePublisher) Publish(_ context.Context, channel string, payload any) error {
	f.channel, f.payload = channel, payload
	return f.err
}

func TestNotifierConvertsDomainAlert(t *testing.T) {
	publisher := &fakePublisher{}
	value := alert.New(
		alert.TypeSnapshotOutbox,
		alert.StateFiring,
		notify.EventLevelError,
		"market",
		"snapshot-outbox",
		"Snapshot Outbox unhealthy",
		"pending rows exceeded the freshness window",
		1234,
	)
	value.Data = map[string]any{"pending": int64(12)}
	if err := alert.Notify(context.Background(), New(publisher), value); err != nil {
		t.Fatal(err)
	}
	if publisher.channel != notify.Channel {
		t.Fatalf("channel=%q", publisher.channel)
	}
	event, ok := publisher.payload.(notify.Event)
	if !ok {
		t.Fatalf("payload=%T", publisher.payload)
	}
	if event.ID != value.ID || event.Type != alert.TypeSnapshotOutbox ||
		event.Level != notify.EventLevelError || event.BizNo != "snapshot-outbox" {
		t.Fatalf("event=%+v", event)
	}
}

func TestResolvedAlertUsesInfoLevel(t *testing.T) {
	publisher := &fakePublisher{}
	value := alert.New(
		alert.TypePriceEngineInput,
		alert.StateResolved,
		notify.EventLevelError,
		"market",
		"price-engine-input",
		"Price Engine recovered",
		"inputs are available",
		1234,
	)
	if err := alert.Notify(context.Background(), New(publisher), value); err != nil {
		t.Fatal(err)
	}
	event := publisher.payload.(notify.Event)
	if event.Level != notify.EventLevelInfo {
		t.Fatalf("resolved level=%q", event.Level)
	}
}

func TestNotifierReturnsPublisherAndConfigurationErrors(t *testing.T) {
	value := alert.New(
		alert.TypeSnapshotOutbox,
		alert.StateFiring,
		"error",
		"market",
		"key",
		"title",
		"message",
		1234,
	)
	if err := alert.Notify(context.Background(), New(nil), value); err == nil {
		t.Fatal("expected missing publisher error")
	}
	publisherErr := errors.New("kafka unavailable")
	if err := alert.Notify(
		context.Background(),
		New(&fakePublisher{err: publisherErr}),
		value,
	); !errors.Is(err, publisherErr) {
		t.Fatalf("publisher error=%v", err)
	}
}
