package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

type Config struct {
	Brokers        []string
	ClientID       string `yaml:"ClientID,optional"`
	GroupID        string `yaml:"GroupID,optional"`
	MinBytes       int
	MaxBytes       int
	MaxAttempts    int
	RetryBackoffMs int64
}

type Message struct {
	Channel string
	Key     []byte
	Payload string
	Headers map[string]string
}

type Handler func(context.Context, Message) error

type Publisher struct {
	writer *kafka.Writer
}

type Subscriber struct {
	config          Config
	startFromLatest bool
}

type keyedBalancer struct {
	hash  kafka.Hash
	least kafka.LeastBytes
}

// ForService assigns the Kafka client identity from the go-zero service name.
// Consumer group IDs remain owned by each concrete subscriber.
func ForService(config Config, serviceName string) Config {
	clientID := strings.ToLower(strings.TrimSpace(serviceName))
	if base, _, found := strings.Cut(clientID, "."); found {
		clientID = base
	}
	config.ClientID = clientID
	return config
}

func (b *keyedBalancer) Balance(message kafka.Message, partitions ...int) int {
	if len(message.Key) > 0 {
		return b.hash.Balance(message, partitions...)
	}
	return b.least.Balance(message, partitions...)
}

func NewPublisher(config Config) (*Publisher, error) {
	brokers, err := validateConfig(config)
	if err != nil {
		return nil, err
	}
	return &Publisher{writer: &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &keyedBalancer{},
		RequiredAcks:           kafka.RequireAll,
		Async:                  false,
		// Single business events are published synchronously. kafka-go defaults
		// to a one-second batch wait, which would make two sequential publishes
		// consume the default two-second RPC deadline.
		BatchTimeout:           10 * time.Millisecond,
		AllowAutoTopicCreation: false,
		Transport:              &kafka.Transport{ClientID: strings.TrimSpace(config.ClientID)},
	}}, nil
}

func MustNewPublisher(config Config) *Publisher {
	publisher, err := NewPublisher(config)
	if err != nil {
		panic(err)
	}
	return publisher
}

func NewSubscriber(config Config, groupID string) (*Subscriber, error) {
	config.GroupID = strings.TrimSpace(groupID)
	if config.GroupID == "" {
		return nil, errors.New("mq consumer group is required")
	}
	if _, err := validateConfig(config); err != nil {
		return nil, err
	}
	return &Subscriber{config: config}, nil
}

func MustNewSubscriber(config Config, groupID string) *Subscriber {
	subscriber, err := NewSubscriber(config, groupID)
	if err != nil {
		panic(err)
	}
	return subscriber
}

// NewBroadcastSubscriber creates a subscriber for live fan-out consumers such as
// WebSocket gateways. A newly created group starts at the end of the topic, while
// an existing group continues from its committed offset.
func NewBroadcastSubscriber(config Config, groupID string) (*Subscriber, error) {
	subscriber, err := NewSubscriber(config, groupID)
	if err != nil {
		return nil, err
	}
	subscriber.startFromLatest = true
	return subscriber, nil
}

func MustNewBroadcastSubscriber(config Config, groupID string) *Subscriber {
	subscriber, err := NewBroadcastSubscriber(config, groupID)
	if err != nil {
		panic(err)
	}
	return subscriber
}

func (p *Publisher) Publish(ctx context.Context, channel string, payload any) error {
	return p.PublishKey(ctx, channel, nil, payload)
}

// PublishKey publishes a message with a stable partitioning key. Events that
// require per-entity ordering must use the same key for that entity.
func (p *Publisher) PublishKey(ctx context.Context, channel string, key []byte, payload any) error {
	if p == nil || p.writer == nil {
		return errors.New("mq publisher is nil")
	}
	topic := Topic(channel)
	if topic == "" {
		return errors.New("mq channel is required")
	}
	data, err := encodePayload(payload)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{Topic: topic, Key: append([]byte(nil), key...), Value: data, Time: time.Now()})
}

func (p *Publisher) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

func (s *Subscriber) Subscribe(ctx context.Context, channel string, handler Handler) error {
	if s == nil || handler == nil {
		return errors.New("mq subscriber and handler are required")
	}
	brokers, err := validateConfig(s.config)
	if err != nil {
		return err
	}
	minBytes, maxBytes := s.config.MinBytes, s.config.MaxBytes
	if minBytes <= 0 {
		minBytes = 1
	}
	if maxBytes <= 0 {
		maxBytes = 10 << 20
	}
	topic := Topic(channel)
	if topic == "" {
		return errors.New("mq channel is required")
	}
	startOffset := kafka.FirstOffset
	if s.startFromLatest {
		startOffset = kafka.LastOffset
	}
	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: brokers, GroupID: s.config.GroupID, Topic: topic, MinBytes: minBytes, MaxBytes: maxBytes, CommitInterval: 0, StartOffset: startOffset, Dialer: &kafka.Dialer{ClientID: strings.TrimSpace(s.config.ClientID)}})
	defer reader.Close()
	for {
		record, fetchErr := reader.FetchMessage(ctx)
		if fetchErr != nil {
			return fetchErr
		}
		message := Message{Channel: channel, Key: record.Key, Payload: string(record.Value), Headers: decodeHeaders(record.Headers)}
		if handleErr := s.handleWithRetry(ctx, handler, message); handleErr != nil {
			if dlqErr := publishDeadLetter(ctx, brokers, record, s.config.GroupID, handleErr); dlqErr != nil {
				return fmt.Errorf("publish dead letter: %w (handler: %v)", dlqErr, handleErr)
			}
		}
		if err = reader.CommitMessages(ctx, record); err != nil {
			return err
		}
	}
}

func (s *Subscriber) handleWithRetry(ctx context.Context, handler Handler, message Message) error {
	attempts := s.config.MaxAttempts
	backoff := time.Duration(s.config.RetryBackoffMs) * time.Millisecond
	if attempts <= 0 {
		attempts = 5
	}
	if backoff <= 0 {
		backoff = time.Second
	}
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err = handler(ctx, message); err == nil {
			return nil
		}
		if attempt < attempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff * time.Duration(attempt)):
			}
		}
	}
	return err
}

func Decode(message Message, target any) error {
	return json.Unmarshal([]byte(message.Payload), target)
}

func Topic(channel string) string {
	replacer := strings.NewReplacer(":", ".", "/", ".", " ", "-")
	return replacer.Replace(strings.TrimSpace(channel))
}

func validateConfig(config Config) ([]string, error) {
	brokers := make([]string, 0, len(config.Brokers))
	for _, broker := range config.Brokers {
		if broker = strings.TrimSpace(broker); broker != "" {
			brokers = append(brokers, broker)
		}
	}
	if len(brokers) == 0 {
		return nil, errors.New("mq kafka brokers are required")
	}
	return brokers, nil
}

func encodePayload(payload any) ([]byte, error) {
	switch value := payload.(type) {
	case nil:
		return nil, nil
	case string:
		return []byte(value), nil
	case []byte:
		return value, nil
	case json.RawMessage:
		return value, nil
	default:
		return json.Marshal(value)
	}
}

func decodeHeaders(headers []kafka.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for _, header := range headers {
		result[header.Key] = string(header.Value)
	}
	return result
}

func publishDeadLetter(ctx context.Context, brokers []string, record kafka.Message, groupID string, cause error) error {
	headers := append([]kafka.Header{}, record.Headers...)
	headers = append(headers, kafka.Header{Key: "x-consumer-group", Value: []byte(groupID)}, kafka.Header{Key: "x-error", Value: []byte(cause.Error())})
	writer := &kafka.Writer{Addr: kafka.TCP(brokers...), RequiredAcks: kafka.RequireAll, Async: false, AllowAutoTopicCreation: false}
	defer writer.Close()
	return writer.WriteMessages(ctx, kafka.Message{Topic: record.Topic + ".dlq", Key: record.Key, Value: record.Value, Headers: headers, Time: time.Now()})
}
