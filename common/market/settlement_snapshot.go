package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

const (
	authoritativeSnapshotTTL       = 365 * 24 * time.Hour
	authoritativeLatestSnapshotTTL = 7 * 24 * time.Hour
)

var setLatestAuthoritativeSnapshotScript = redis.NewScript(`
local incoming = cjson.decode(ARGV[1])
local currentRaw = redis.call('GET', KEYS[1])
if currentRaw then
  local current = cjson.decode(currentRaw)
  local currentSource = tonumber(current.sourceTimestamp) or 0
  local incomingSource = tonumber(incoming.sourceTimestamp) or 0
  local currentRevision = tonumber(current.revision) or 0
  local incomingRevision = tonumber(incoming.revision) or 0
  local currentSnapshot = tonumber(current.snapshotTimestamp) or 0
  local incomingSnapshot = tonumber(incoming.snapshotTimestamp) or 0
  if currentSource > incomingSource or
     (currentSource == incomingSource and currentRevision > incomingRevision) or
     (currentSource == incomingSource and currentRevision == incomingRevision and currentSnapshot >= incomingSnapshot) then
    return currentSource
  end
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
return tonumber(incoming.sourceTimestamp) or 0
`)

func (b *MarketDataCache) LockPriceSnapshot(ctx context.Context, kind string, msg ClientMessage, maxAge time.Duration) (*SettlementSnapshot, error) {
	items, err := b.ReadMany(ctx, []ClientMessage{msg})
	if err != nil {
		return nil, err
	}
	if len(items) != 1 {
		return nil, errors.New("settlement price unavailable")
	}
	q, ok := items[0].Payload.(*QuotePayload)
	if !ok || q == nil || q.LastPrice <= 0 || q.Ts <= 0 {
		return nil, errors.New("invalid settlement quote")
	}
	now := time.Now().UnixMilli()
	if q.Ts > now+1000 || now-q.Ts > maxAge.Milliseconds() {
		return nil, errors.New("stale settlement quote")
	}
	msg = NormalizeClientMessage(msg)
	s := &SettlementSnapshot{Kind: kind, CategoryCode: msg.CategoryCode, Market: msg.Market, Symbol: msg.Symbol, Price: fmt.Sprintf("%.18g", q.LastPrice), Source: msg.Market, SourceTimestamp: q.Ts, SnapshotTimestamp: now, Revision: q.Ts, Confirmed: true}
	s.SnapshotID = snapshotDigest(s)
	if err := b.PutSettlementSnapshot(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (b *MarketDataCache) PutSettlementSnapshot(ctx context.Context, s *SettlementSnapshot) error {
	if s == nil || !s.Confirmed || s.SourceTimestamp <= 0 {
		return errors.New("unconfirmed settlement snapshot")
	}
	if s.SnapshotTimestamp <= 0 {
		s.SnapshotTimestamp = time.Now().UnixMilli()
	}
	if s.Revision <= 0 {
		s.Revision = s.SourceTimestamp
	}
	if s.SnapshotID == "" {
		s.SnapshotID = snapshotDigest(s)
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("market:settlement:v1:%s", s.SnapshotID)
	return b.rdb.SetNX(ctx, key, raw, 30*24*time.Hour).Err()
}

// PublishAuthoritativeQuote archives an immutable source-owned quote. Only the
// market-data producer may call this; consumers must query the archive and may
// not promote their local quote cache to an authoritative snapshot.
func (b *MarketDataCache) PublishAuthoritativeQuote(ctx context.Context, msg ClientMessage, q *QuotePayload) (*SettlementSnapshot, error) {
	s, err := BuildAuthoritativeQuoteSnapshot(msg, q)
	if err != nil {
		return nil, err
	}
	if err = b.PublishAuthoritativeSnapshot(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

func BuildAuthoritativeQuoteSnapshot(msg ClientMessage, q *QuotePayload) (*SettlementSnapshot, error) {
	msg = NormalizeClientMessage(msg)
	if q == nil || q.Ts <= 0 || strings.TrimSpace(q.LastPriceText) == "" || strings.TrimSpace(q.Authority) == "" {
		return nil, errors.New("authoritative quote metadata is incomplete")
	}
	priceText := strings.TrimSpace(q.LastPriceText)
	price, err := decimal.NewFromString(priceText)
	if err != nil || !price.IsPositive() {
		return nil, errors.New("authoritative quote price is invalid")
	}
	if err = validateArchiveDecimal(price); err != nil {
		return nil, err
	}
	s := &SettlementSnapshot{
		Kind:              "FINAL_QUOTE",
		CategoryCode:      msg.CategoryCode,
		Market:            msg.Market,
		Symbol:            msg.Symbol,
		Price:             priceText,
		Source:            msg.Market,
		SourceTimestamp:   q.Ts,
		SnapshotTimestamp: time.Now().UnixMilli(),
		Revision:          q.Ts,
		FormulaVersion:    "source-quote-v1",
		Authority:         strings.TrimSpace(q.Authority),
		Confirmed:         true,
	}
	s.SnapshotID = snapshotDigest(s)
	return s, nil
}

func validateArchiveDecimal(value decimal.Decimal) error {
	if value.Exponent() < -30 {
		return errors.New("authoritative quote exceeds 30 decimal places")
	}
	integerDigits := len(value.Abs().Truncate(0).StringFixed(0))
	if integerDigits > 35 {
		return errors.New("authoritative quote exceeds 35 integer digits")
	}
	return nil
}

func (b *MarketDataCache) PublishAuthoritativeSnapshot(ctx context.Context, s *SettlementSnapshot) error {
	if s == nil || s.SnapshotID == "" || !s.Confirmed || s.Authority == "" || s.SourceTimestamp <= 0 {
		return errors.New("invalid authoritative snapshot")
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	msg := ClientMessage{Topic: TopicQuote, CategoryCode: s.CategoryCode, Market: s.Market, Symbol: s.Symbol}
	b.mu.RLock()
	hotWindow := b.authoritativeHotWindow
	b.mu.RUnlock()
	if hotWindow <= 0 {
		hotWindow = 30 * time.Minute
	}
	latestKey := authoritativeLatestSnapshotKey(msg, s.Authority, s.Kind)
	hotKey := authoritativeHotSnapshotKey(msg, s.Authority, s.Kind)
	latestSource, err := setLatestAuthoritativeSnapshotScript.Run(ctx, b.rdb, []string{latestKey}, raw, authoritativeLatestSnapshotTTL.Milliseconds()).Int64()
	if err != nil {
		return err
	}
	cutoff := latestSource - hotWindow.Milliseconds()
	pipe := b.rdb.TxPipeline()
	if s.SourceTimestamp >= cutoff {
		pipe.ZAdd(ctx, hotKey, redis.Z{Score: float64(s.SourceTimestamp), Member: raw})
	}
	pipe.ZRemRangeByScore(ctx, hotKey, "-inf", fmt.Sprintf("(%d", cutoff))
	pipe.Expire(ctx, hotKey, hotWindow*2)
	if _, err = pipe.Exec(ctx); err != nil {
		return err
	}
	return nil
}

// FindAuthoritativeQuoteAt returns the newest finalized source quote at or
// before targetTime, bounded by maxLookback.
func (b *MarketDataCache) FindAuthoritativeQuoteAt(ctx context.Context, msg ClientMessage, authority string, targetTime int64, maxLookback time.Duration) (*SettlementSnapshot, error) {
	return b.FindAuthoritativeSnapshotAt(ctx, msg, authority, "FINAL_QUOTE", targetTime, maxLookback)
}

// FindAuthoritativeSnapshotAt reads a purpose-specific immutable snapshot.
// Kind is part of the index so MARK, INDEX, FUNDING and DELIVERY cannot shadow
// one another when they share an authority and product.
func (b *MarketDataCache) FindAuthoritativeSnapshotAt(ctx context.Context, msg ClientMessage, authority, kind string, targetTime int64, maxLookback time.Duration) (*SettlementSnapshot, error) {
	msg = NormalizeClientMessage(msg)
	kind = strings.ToUpper(strings.TrimSpace(kind))
	if targetTime <= 0 || maxLookback <= 0 || strings.TrimSpace(authority) == "" || kind == "" {
		return nil, errors.New("invalid authoritative snapshot query")
	}
	raws, err := b.rdb.ZRevRangeByScore(ctx, authoritativeHotSnapshotKey(msg, authority, kind), &redis.ZRangeBy{Max: fmt.Sprintf("%d", targetTime), Min: fmt.Sprintf("%d", targetTime-maxLookback.Milliseconds()), Offset: 0, Count: 100}).Result()
	if err != nil {
		return nil, err
	}
	if len(raws) == 0 {
		return nil, errors.New("authoritative snapshot unavailable at target time")
	}
	var selected *SettlementSnapshot
	for _, raw := range raws {
		var candidate SettlementSnapshot
		if json.Unmarshal([]byte(raw), &candidate) != nil || !candidate.Confirmed || !strings.EqualFold(candidate.Authority, strings.TrimSpace(authority)) || !strings.EqualFold(candidate.Kind, kind) || candidate.SourceTimestamp > targetTime {
			continue
		}
		revoked, revokeErr := b.rdb.Exists(ctx, authoritativeSnapshotRevocationKey(candidate.SnapshotID)).Result()
		if revokeErr != nil || revoked > 0 {
			continue
		}
		if selected == nil || candidate.SourceTimestamp > selected.SourceTimestamp ||
			(candidate.SourceTimestamp == selected.SourceTimestamp && candidate.Revision > selected.Revision) ||
			(candidate.SourceTimestamp == selected.SourceTimestamp && candidate.Revision == selected.Revision && candidate.SnapshotTimestamp > selected.SnapshotTimestamp) {
			copy := candidate
			selected = &copy
		}
	}
	if selected == nil {
		return nil, errors.New("valid authoritative snapshot unavailable at target time")
	}
	return selected, nil
}

// RevokeAuthoritativeSnapshot publishes an immutable tombstone. Replacement
// identity is retained for audit; normal revision ordering selects it.
func (b *MarketDataCache) RevokeAuthoritativeSnapshot(ctx context.Context, snapshotID, replacementID, reason string) error {
	snapshotID, replacementID, reason = strings.TrimSpace(snapshotID), strings.TrimSpace(replacementID), strings.TrimSpace(reason)
	if snapshotID == "" || reason == "" || snapshotID == replacementID {
		return errors.New("invalid authoritative snapshot revocation")
	}
	raw, err := json.Marshal(map[string]string{"snapshot_id": snapshotID, "replacement_snapshot_id": replacementID, "reason": reason})
	if err != nil {
		return err
	}
	return b.rdb.Set(ctx, authoritativeSnapshotRevocationKey(snapshotID), raw, authoritativeSnapshotTTL).Err()
}

func authoritativeSnapshotRevocationKey(snapshotID string) string {
	return fmt.Sprintf("market:authoritative:v1:revoked:%s", strings.TrimSpace(snapshotID))
}

func authoritativeSnapshotIndex(msg ClientMessage, authority string) string {
	return fmt.Sprintf("market:authoritative:v1:index:%s:%s:%s:%s", strings.ToLower(strings.TrimSpace(authority)), msg.CategoryCode, msg.Market, msg.Symbol)
}

func authoritativeSnapshotKindIndex(msg ClientMessage, authority, kind string) string {
	return fmt.Sprintf("market:authoritative:v2:index:%s:%s:%s:%s:%s", strings.ToLower(strings.TrimSpace(authority)), strings.ToUpper(strings.TrimSpace(kind)), msg.CategoryCode, msg.Market, msg.Symbol)
}

func authoritativeLatestSnapshotKey(msg ClientMessage, authority, kind string) string {
	msg = NormalizeClientMessage(msg)
	return fmt.Sprintf("market:authoritative:v3:latest:%s:%s:%s:%s:%s", strings.ToLower(strings.TrimSpace(authority)), strings.ToUpper(strings.TrimSpace(kind)), msg.CategoryCode, msg.Market, msg.Symbol)
}

func authoritativeHotSnapshotKey(msg ClientMessage, authority, kind string) string {
	msg = NormalizeClientMessage(msg)
	return fmt.Sprintf("market:authoritative:v3:hot:%s:%s:%s:%s:%s", strings.ToLower(strings.TrimSpace(authority)), strings.ToUpper(strings.TrimSpace(kind)), msg.CategoryCode, msg.Market, msg.Symbol)
}

// FindLatestAuthoritativeSnapshot returns the latest product snapshot without
// retaining the full immutable history in Redis.
func (b *MarketDataCache) FindLatestAuthoritativeSnapshot(ctx context.Context, msg ClientMessage, authority, kind string) (*SettlementSnapshot, error) {
	raw, err := b.rdb.Get(ctx, authoritativeLatestSnapshotKey(msg, authority, kind)).Bytes()
	if err != nil {
		return nil, err
	}
	var snapshot SettlementSnapshot
	if err = json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	if !snapshot.Confirmed || snapshot.SnapshotID == "" {
		return nil, errors.New("invalid latest authoritative snapshot")
	}
	return &snapshot, nil
}

// CleanupLegacyAuthoritativeCache removes a bounded SCAN batch of the v1/v2
// cache layout. Revocation tombstones are retained for correctness.
func (b *MarketDataCache) CleanupLegacyAuthoritativeCache(ctx context.Context, cursor uint64, count int64) (uint64, int64, error) {
	if count <= 0 || count > 5000 {
		count = 500
	}
	var deleted int64
	next, keys, err := scanLegacyAuthoritativeKeys(ctx, b.rdb, cursor, count)
	if err != nil {
		return cursor, 0, err
	}
	if len(keys) > 0 {
		deleted, err = b.rdb.Unlink(ctx, keys...).Result()
	}
	return next, deleted, err
}

func scanLegacyAuthoritativeKeys(ctx context.Context, rdb *redis.Client, cursor uint64, count int64) (uint64, []string, error) {
	keys, next, err := rdb.Scan(ctx, cursor, "market:authoritative:v[12]:*", count).Result()
	if err != nil {
		return cursor, nil, err
	}
	filtered := keys[:0]
	for _, key := range keys {
		if strings.Contains(key, ":revoked:") {
			continue
		}
		filtered = append(filtered, key)
	}
	return next, filtered, nil
}

func (b *MarketDataCache) GetSettlementSnapshot(ctx context.Context, id string) (*SettlementSnapshot, error) {
	raw, err := b.rdb.Get(ctx, fmt.Sprintf("market:settlement:v1:%s", id)).Bytes()
	if err != nil {
		return nil, err
	}
	var s SettlementSnapshot
	if err = json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func snapshotDigest(s *SettlementSnapshot) string {
	copy := *s
	copy.SnapshotID = ""
	// Reception time is audit metadata, not source identity. Re-reading the same
	// revision must resolve to the same immutable snapshot ID.
	copy.SnapshotTimestamp = 0
	raw, _ := json.Marshal(copy)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
