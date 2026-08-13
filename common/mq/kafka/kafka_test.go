package mq

import (
	"context"
	"errors"
	"testing"

	kafka "github.com/segmentio/kafka-go"
)

func TestTopicNormalizesLegacyRedisChannel(t *testing.T) {
	if got, want := Topic(" appforge:chat/app events "), "appforge.chat.app-events"; got != want {
		t.Fatalf("Topic() = %q, want %q", got, want)
	}
}

func TestForServiceDerivesClientID(t *testing.T) {
	config := ForService(Config{ClientID: "configured-value"}, " Trade.RPC ")
	if config.ClientID != "trade" {
		t.Fatalf("ClientID = %q, want trade", config.ClientID)
	}
}

func TestEncodePayloadPreservesBytesAndEncodesObjects(t *testing.T) {
	raw := []byte(`{"id":1}`)
	got, err := encodePayload(raw)
	if err != nil || string(got) != string(raw) {
		t.Fatalf("encode bytes = %q, %v", got, err)
	}
	got, err = encodePayload(struct {
		ID int `json:"id"`
	}{ID: 1})
	if err != nil || string(got) != string(raw) {
		t.Fatalf("encode object = %q, %v", got, err)
	}
}

func TestHandleWithRetry(t *testing.T) {
	subscriber := &Subscriber{config: Config{MaxAttempts: 3, RetryBackoffMs: 1}}
	attempts := 0
	err := subscriber.handleWithRetry(context.Background(), func(context.Context, Message) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary")
		}
		return nil
	}, Message{})
	if err != nil || attempts != 3 {
		t.Fatalf("handleWithRetry() err=%v attempts=%d", err, attempts)
	}
}

func TestBroadcastSubscriberStartsFromLatest(t *testing.T) {
	subscriber, err := NewBroadcastSubscriber(Config{Brokers: []string{"localhost:9092"}}, "gateway-1")
	if err != nil {
		t.Fatal(err)
	}
	if !subscriber.startFromLatest {
		t.Fatal("broadcast subscriber must start from latest for a new consumer group")
	}
}

func TestKeyedBalancerKeepsEntityOnSamePartition(t *testing.T) {
	balancer := &keyedBalancer{}
	partitions := []int{0, 1, 2, 3}
	first := balancer.Balance(kafka.Message{Key: []byte("fill-1")}, partitions...)
	second := balancer.Balance(kafka.Message{Key: []byte("fill-1")}, partitions...)
	if first != second {
		t.Fatalf("same key selected partitions %d and %d", first, second)
	}
}
