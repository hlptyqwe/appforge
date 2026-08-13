package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheEnvelope struct {
	Topic        string          `json:"topic"`
	CategoryCode string          `json:"categoryCode"`
	Symbol       string          `json:"symbol"`
	Market       string          `json:"market,omitempty"`
	Interval     string          `json:"interval,omitempty"`
	Payload      json.RawMessage `json:"payload"`
	Source       string          `json:"source,omitempty"`
	Revision     int64           `json:"revision,omitempty"`
}

var setVersionedKlineScript = redis.NewScript(`
local oldPriority = tonumber(redis.call('HGET', KEYS[2], 'priority') or '0')
local oldRevision = tonumber(redis.call('HGET', KEYS[2], 'revision') or '0')
local priority = tonumber(ARGV[2])
local revision = tonumber(ARGV[3])
if oldPriority > priority or (oldPriority == priority and oldRevision > revision) then
  return 0
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[4])
redis.call('HSET', KEYS[2], 'priority', priority, 'revision', revision)
redis.call('PEXPIRE', KEYS[2], ARGV[5])
return 1
`)

var setVersionedRealtimeScript = redis.NewScript(`
local oldTs = tonumber(redis.call('HGET', KEYS[2], 'source_ts') or '0')
local sourceTs = tonumber(ARGV[2])
if sourceTs <= 0 or oldTs >= sourceTs then return 0 end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[3])
redis.call('HSET', KEYS[2], 'source_ts', sourceTs)
redis.call('PEXPIRE', KEYS[2], ARGV[3])
return 1
`)

type MarketDataCache struct {
	rdb                    *redis.Client
	mu                     sync.RWMutex
	klineStaleTTL          time.Duration
	authoritativeHotWindow time.Duration
	quoteHandler           func(context.Context, ClientMessage, *QuotePayload) error
	tickHandler            func(context.Context, ClientMessage, *TickPayload)
}

type CachedMarketData struct {
	Message ClientMessage
	Payload any
	Version string
}

func NewMarketDataCache(rdb *redis.Client) *MarketDataCache {
	return &MarketDataCache{rdb: rdb, klineStaleTTL: 30 * time.Second, authoritativeHotWindow: 30 * time.Minute}
}

// SetAuthoritativeHotWindow limits authoritative snapshot history retained in
// Redis. MySQL remains the permanent archive for older snapshots.
func (b *MarketDataCache) SetAuthoritativeHotWindow(window time.Duration) {
	if window <= 0 {
		return
	}
	b.mu.Lock()
	b.authoritativeHotWindow = window
	b.mu.Unlock()
}

func (b *MarketDataCache) SetKlineStaleTTL(ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	b.mu.Lock()
	b.klineStaleTTL = ttl
	b.mu.Unlock()
}

func (b *MarketDataCache) Set(ctx context.Context, msg ClientMessage, payload any) error {
	msg = NormalizeClientMessage(msg)
	if quote, ok := payload.(*QuotePayload); ok && quote != nil && quote.Authority != "" {
		b.mu.RLock()
		handler := b.quoteHandler
		b.mu.RUnlock()
		if handler == nil {
			return fmt.Errorf("authoritative quote handler is not configured")
		}
		if err := handler(ctx, msg, quote); err != nil {
			return err
		}
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	env := CacheEnvelope{
		Topic:        string(msg.Topic),
		CategoryCode: msg.CategoryCode,
		Symbol:       msg.Symbol,
		Market:       msg.Market,
		Interval:     msg.Interval,
		Payload:      raw,
	}
	if kline, ok := payload.(*KlinePayload); ok && kline != nil {
		env.Source, env.Revision = kline.Source, kline.Revision
	}

	bs, err := json.Marshal(env)
	if err != nil {
		return err
	}

	if msg.Topic == TopicKline {
		priority := klineCachePriority(env.Source)
		if env.Revision <= 0 {
			env.Revision = time.Now().UnixMilli()
		}
		key := marketDataKey(msg)
		b.mu.RLock()
		staleTTL := b.klineStaleTTL
		b.mu.RUnlock()
		if _, err := setVersionedKlineScript.Run(ctx, b.rdb, []string{key, key + ":meta"}, bs,
			priority, env.Revision, marketDataTTL(msg.Topic).Milliseconds(), staleTTL.Milliseconds()).Result(); err != nil {
			return err
		}
	} else {
		sourceTs := int64(0)
		switch v := payload.(type) {
		case *QuotePayload:
			if v != nil {
				sourceTs = v.Ts
			}
		case *TickPayload:
			if v != nil {
				sourceTs = v.Ts
			}
		}
		if sourceTs > 0 {
			key := marketDataKey(msg)
			accepted, runErr := setVersionedRealtimeScript.Run(ctx, b.rdb, []string{key, key + ":meta"}, raw, sourceTs, marketDataTTL(msg.Topic).Milliseconds()).Int()
			if runErr != nil {
				return runErr
			}
			if accepted == 0 {
				return nil
			}
		} else if err := b.rdb.Set(ctx, marketDataKey(msg), raw, marketDataTTL(msg.Topic)).Err(); err != nil {
			return err
		}
	}
	if quote, ok := payload.(*QuotePayload); ok && quote != nil {
		if quote.Authority == "" {
			b.mu.RLock()
			handler := b.quoteHandler
			b.mu.RUnlock()
			if handler != nil {
				if err := handler(ctx, msg, quote); err != nil {
					return err
				}
			}
		}
	}
	if tick, ok := payload.(*TickPayload); ok && tick != nil {
		b.mu.RLock()
		handler := b.tickHandler
		b.mu.RUnlock()
		if handler != nil {
			handler(ctx, msg, tick)
		}
	}
	return nil
}

func klineCachePriority(source string) int {
	switch source {
	case "market_ws":
		return 300
	case "derived":
		return 200
	default:
		return 100
	}
}

func (b *MarketDataCache) SetTickHandler(handler func(context.Context, ClientMessage, *TickPayload)) {
	b.mu.Lock()
	b.tickHandler = handler
	b.mu.Unlock()
}

func (b *MarketDataCache) SetQuoteHandler(handler func(context.Context, ClientMessage, *QuotePayload) error) {
	b.mu.Lock()
	b.quoteHandler = handler
	b.mu.Unlock()
}

func (b *MarketDataCache) ReadMany(ctx context.Context, msgs []ClientMessage) ([]CachedMarketData, error) {
	if len(msgs) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		keys = append(keys, marketDataKey(msg))
	}
	values, err := b.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	out := make([]CachedMarketData, 0, len(values))
	for i, value := range values {
		raw, ok := value.(string)
		if !ok || raw == "" {
			continue
		}
		payloadRaw := json.RawMessage(raw)
		if msgs[i].Topic == TopicKline {
			var env CacheEnvelope
			if err := json.Unmarshal([]byte(raw), &env); err != nil {
				continue
			}
			payloadRaw = env.Payload
		}
		payload, err := decodeMarketDataPayload(msgs[i].Topic, payloadRaw)
		if err != nil {
			continue
		}
		out = append(out, CachedMarketData{Message: msgs[i], Payload: payload, Version: raw})
	}
	return out, nil
}

func marketDataKey(msg ClientMessage) string {
	msg = NormalizeClientMessage(msg)
	if msg.Topic == TopicKline {
		return fmt.Sprintf("market:v1:kline:%s:%s:%s:%s", msg.CategoryCode, msg.Market, msg.Symbol, msg.Interval)
	}
	return fmt.Sprintf("market:%s:%s:%s:%s", msg.Topic, msg.CategoryCode, msg.Market, msg.Symbol)
}

func marketDataTTL(topic Topic) time.Duration {
	switch topic {
	case TopicDepth:
		return 5 * time.Minute
	case TopicKline:
		return 24 * time.Hour
	default:
		return 30 * time.Minute
	}
}

func decodeMarketDataPayload(topic Topic, raw json.RawMessage) (any, error) {
	switch topic {
	case TopicQuote:
		var v QuotePayload
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return &v, nil

	case TopicTick:
		var v TickPayload
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return &v, nil

	case TopicDepth:
		var v DepthPayload
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return &v, nil

	case TopicKline:
		var v KlinePayload
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return &v, nil
	}

	return nil, nil
}
