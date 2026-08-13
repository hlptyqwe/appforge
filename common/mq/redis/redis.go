// Package bus provides the legacy Redis Pub/Sub adapter. New business code
// should use appforge/common/mq/kafka; this package remains for compatibility.
package mq

import (
	"context"
	"encoding/json"
	"fmt"

	v9 "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type Message struct {
	Channel string
	Payload string
}

type Handler func(context.Context, Message) error

type Publisher struct{ rds *redis.Redis }
type Subscriber struct{ client *v9.Client }

func NewPublisher(rds *redis.Redis) *Publisher { return &Publisher{rds: rds} }
func NewPublisherFromRedisConf(conf redis.RedisConf) *Publisher {
	return NewPublisher(redis.MustNewRedis(conf))
}
func (p *Publisher) Publish(ctx context.Context, channel string, payload any) error {
	if p == nil {
		return fmt.Errorf("bus publisher is nil")
	}
	return Publish(ctx, p.rds, channel, payload)
}
func Publish(ctx context.Context, rds *redis.Redis, channel string, payload any) error {
	if rds == nil {
		return fmt.Errorf("bus redis publisher is nil")
	}
	if channel == "" {
		return fmt.Errorf("bus channel is empty")
	}
	data, err := encodePayload(payload)
	if err != nil {
		return err
	}
	_, err = rds.PublishCtx(ctx, channel, data)
	return err
}

func NewSubscriber(client *v9.Client) *Subscriber { return &Subscriber{client: client} }
func NewSubscriberFromRedisConf(conf redis.RedisConf) *Subscriber {
	return NewSubscriber(NewGoRedisClient(conf))
}
func (s *Subscriber) Subscribe(ctx context.Context, channel string, handler Handler) error {
	if s == nil {
		return fmt.Errorf("bus subscriber is nil")
	}
	return Subscribe(ctx, s.client, channel, handler)
}
func Subscribe(ctx context.Context, client *v9.Client, channel string, handler Handler) error {
	if client == nil {
		return fmt.Errorf("bus redis subscriber is nil")
	}
	if channel == "" {
		return fmt.Errorf("bus channel is empty")
	}
	if handler == nil {
		return fmt.Errorf("bus handler is nil")
	}
	pubsub := client.Subscribe(ctx, channel)
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		return err
	}
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if err := handler(ctx, Message{Channel: msg.Channel, Payload: msg.Payload}); err != nil {
				return err
			}
		}
	}
}

func NewGoRedisClient(conf redis.RedisConf) *v9.Client {
	return v9.NewClient(&v9.Options{Addr: conf.Host, Username: conf.User, Password: conf.Pass})
}
func encodePayload(payload any) (any, error) {
	switch value := payload.(type) {
	case nil:
		return "", nil
	case string:
		return value, nil
	case []byte:
		return value, nil
	case json.RawMessage:
		return value, nil
	default:
		return json.Marshal(value)
	}
}
func Decode(message Message, target any) error {
	return json.Unmarshal([]byte(message.Payload), target)
}
