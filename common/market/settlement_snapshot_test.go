package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSettlementSnapshotDigestBindsRevisionAndFormula(t *testing.T) {
	base := &SettlementSnapshot{Kind: "FUNDING", MarkPrice: "100", IndexPrice: "99", FundingRate: "0.01", SourceTimestamp: 1000, SnapshotTimestamp: 1001, Revision: 7, FormulaVersion: "premium-v1", Confirmed: true}
	a := snapshotDigest(base)
	copy := *base
	copy.Revision++
	if a == snapshotDigest(&copy) {
		t.Fatal("revision must change snapshot id")
	}
	copy = *base
	copy.FormulaVersion = "premium-v2"
	if a == snapshotDigest(&copy) {
		t.Fatal("formula version must change snapshot id")
	}
	copy = *base
	copy.SnapshotTimestamp++
	if a != snapshotDigest(&copy) {
		t.Fatal("read time must not change source snapshot id")
	}
}

func TestAuthoritativeQuoteArchivePreservesDecimalAndHistoricalTime(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache := NewMarketDataCache(client)
	ctx := context.Background()
	msg := ClientMessage{Topic: TopicQuote, CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT"}
	for _, quote := range []*QuotePayload{
		{LastPrice: 1.2345678901234567, LastPriceText: "1.234567890123456789", Ts: 1000, Authority: "itick-ws"},
		{LastPrice: 2, LastPriceText: "2.000000000000000001", Ts: 2000, Authority: "itick-ws"},
	} {
		if _, err := cache.PublishAuthoritativeQuote(ctx, msg, quote); err != nil {
			t.Fatal(err)
		}
	}
	got, err := cache.FindAuthoritativeQuoteAt(ctx, msg, "itick-ws", 1500, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got.Price != "1.234567890123456789" || got.SourceTimestamp != 1000 || !got.Confirmed {
		t.Fatalf("unexpected historical snapshot: %#v", got)
	}
}

func TestAuthoritativeQuoteHandlerFailureDoesNotPublishRealtimeCache(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache := NewMarketDataCache(client)
	cache.SetQuoteHandler(func(context.Context, ClientMessage, *QuotePayload) error {
		return context.DeadlineExceeded
	})
	msg := ClientMessage{Topic: TopicQuote, CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT"}
	err := cache.Set(context.Background(), msg, &QuotePayload{LastPrice: 1, LastPriceText: "1", Ts: 1000, Authority: "itick-ws"})
	if err == nil {
		t.Fatal("expected durable archive handler failure")
	}
	items, readErr := cache.ReadMany(context.Background(), []ClientMessage{msg})
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(items) != 0 {
		t.Fatal("realtime cache must not be published before durable archive")
	}
}

func TestAuthoritativeSnapshotIndexSeparatesKinds(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache := NewMarketDataCache(client)
	ctx := context.Background()
	msg := ClientMessage{Topic: TopicQuote, CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT"}
	for _, snapshot := range []*SettlementSnapshot{
		{SnapshotID: "mark-1", Authority: "price-engine", Kind: "MARK", CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT", Price: "100", SourceTimestamp: 1000, SnapshotTimestamp: 1001, Revision: 1000, Confirmed: true},
		{SnapshotID: "funding-1", Authority: "price-engine", Kind: "FUNDING", CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT", Price: "0.001", SourceTimestamp: 1100, SnapshotTimestamp: 1101, Revision: 1100, Confirmed: true},
	} {
		if err := cache.PublishAuthoritativeSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	mark, err := cache.FindAuthoritativeSnapshotAt(ctx, msg, "price-engine", "MARK", 1200, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	funding, err := cache.FindAuthoritativeSnapshotAt(ctx, msg, "price-engine", "FUNDING", 1200, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if mark.SnapshotID != "mark-1" || funding.SnapshotID != "funding-1" {
		t.Fatalf("snapshot kinds crossed indexes: mark=%s funding=%s", mark.SnapshotID, funding.SnapshotID)
	}
}

func TestAuthoritativeSnapshotSelectsHighestRevisionAtSameSourceTime(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache := NewMarketDataCache(client)
	ctx := context.Background()
	msg := ClientMessage{Topic: TopicQuote, CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT"}
	for _, snapshot := range []*SettlementSnapshot{
		{SnapshotID: "mark-rev-1", Authority: "price-engine", Kind: "MARK", CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT", Price: "100", SourceTimestamp: 1000, SnapshotTimestamp: 1001, Revision: 1, Confirmed: true},
		{SnapshotID: "mark-rev-2", Authority: "price-engine", Kind: "MARK", CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT", Price: "101", SourceTimestamp: 1000, SnapshotTimestamp: 1002, Revision: 2, Confirmed: true},
	} {
		if err := cache.PublishAuthoritativeSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	got, err := cache.FindAuthoritativeSnapshotAt(ctx, msg, "price-engine", "MARK", 1000, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got.SnapshotID != "mark-rev-2" {
		t.Fatalf("expected highest revision, got %s", got.SnapshotID)
	}
}

func TestAuthoritativeSnapshotSkipsRevokedRevision(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache := NewMarketDataCache(client)
	ctx := context.Background()
	msg := ClientMessage{Topic: TopicQuote, CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT"}
	for _, snapshot := range []*SettlementSnapshot{
		{SnapshotID: "mark-valid", Authority: "price-engine", Kind: "MARK", CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT", Price: "100", SourceTimestamp: 1000, SnapshotTimestamp: 1001, Revision: 1, Confirmed: true},
		{SnapshotID: "mark-bad", Authority: "price-engine", Kind: "MARK", CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT", Price: "999", SourceTimestamp: 1000, SnapshotTimestamp: 1002, Revision: 2, Confirmed: true},
	} {
		if err := cache.PublishAuthoritativeSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	if err := cache.RevokeAuthoritativeSnapshot(ctx, "mark-bad", "", "bad source input"); err != nil {
		t.Fatal(err)
	}
	got, err := cache.FindAuthoritativeSnapshotAt(ctx, msg, "price-engine", "MARK", 1000, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got.SnapshotID != "mark-valid" {
		t.Fatalf("expected fallback to valid revision, got %s", got.SnapshotID)
	}
}

func TestAuthoritativeSnapshotCacheUsesBoundedV3Layout(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache := NewMarketDataCache(client)
	cache.SetAuthoritativeHotWindow(time.Minute)
	ctx := context.Background()
	msg := ClientMessage{Topic: TopicQuote, CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT"}
	first := &SettlementSnapshot{SnapshotID: "first", Authority: "itick-ws", Kind: "FINAL_QUOTE", CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT", Price: "100", SourceTimestamp: 1000, SnapshotTimestamp: 1001, Revision: 1000, Confirmed: true}
	second := &SettlementSnapshot{SnapshotID: "second", Authority: "itick-ws", Kind: "FINAL_QUOTE", CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT", Price: "101", SourceTimestamp: 61_001, SnapshotTimestamp: 61_002, Revision: 61_001, Confirmed: true}
	for _, snapshot := range []*SettlementSnapshot{first, second} {
		if err := cache.PublishAuthoritativeSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	if exists := client.Exists(ctx, "market:authoritative:v1:first").Val(); exists != 0 {
		t.Fatal("v3 publisher must not create per-snapshot legacy keys")
	}
	if count := client.ZCard(ctx, authoritativeHotSnapshotKey(msg, "itick-ws", "FINAL_QUOTE")).Val(); count != 1 {
		t.Fatalf("hot snapshot count=%d want=1", count)
	}
	latest, err := cache.FindLatestAuthoritativeSnapshot(ctx, msg, "itick-ws", "FINAL_QUOTE")
	if err != nil {
		t.Fatal(err)
	}
	if latest.SnapshotID != second.SnapshotID {
		t.Fatalf("latest=%s want=%s", latest.SnapshotID, second.SnapshotID)
	}
	if err := cache.PublishAuthoritativeSnapshot(ctx, first); err != nil {
		t.Fatal(err)
	}
	latest, err = cache.FindLatestAuthoritativeSnapshot(ctx, msg, "itick-ws", "FINAL_QUOTE")
	if err != nil {
		t.Fatal(err)
	}
	if latest.SnapshotID != second.SnapshotID {
		t.Fatalf("out-of-order publish regressed latest to %s", latest.SnapshotID)
	}
	if count := client.ZCard(ctx, authoritativeHotSnapshotKey(msg, "itick-ws", "FINAL_QUOTE")).Val(); count != 1 {
		t.Fatalf("out-of-window replay increased hot snapshot count to %d", count)
	}
}

func TestCleanupLegacyAuthoritativeCachePreservesRevocations(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache := NewMarketDataCache(client)
	ctx := context.Background()
	legacyData := "market:authoritative:v1:snapshot-1"
	legacyIndex := "market:authoritative:v2:index:itick-ws:FINAL_QUOTE:crypto:BA:BTCUSDT"
	revocation := "market:authoritative:v1:revoked:snapshot-2"
	for _, key := range []string{legacyData, legacyIndex, revocation} {
		if err := client.Set(ctx, key, "value", time.Hour).Err(); err != nil {
			t.Fatal(err)
		}
	}
	var cursor uint64
	for {
		next, _, err := cache.CleanupLegacyAuthoritativeCache(ctx, cursor, 100)
		if err != nil {
			t.Fatal(err)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	if exists := client.Exists(ctx, legacyData, legacyIndex).Val(); exists != 0 {
		t.Fatalf("legacy cache keys still exist: %d", exists)
	}
	if exists := client.Exists(ctx, revocation).Val(); exists != 1 {
		t.Fatal("revocation tombstone must be preserved")
	}
}

func TestAuthoritativeQuoteRejectsDatabasePrecisionOverflow(t *testing.T) {
	msg := ClientMessage{Topic: TopicQuote, CategoryCode: "crypto", Market: "BA", Symbol: "BTCUSDT"}
	for _, price := range []string{"1.1234567890123456789012345678901", "123456789012345678901234567890123456"} {
		if _, err := BuildAuthoritativeQuoteSnapshot(msg, &QuotePayload{LastPriceText: price, Ts: 1, Authority: "itick-ws"}); err == nil {
			t.Fatalf("expected precision overflow for %s", price)
		}
	}
}
