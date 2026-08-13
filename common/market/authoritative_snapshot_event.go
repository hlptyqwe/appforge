package cache

const (
	AuthoritativeSnapshotTopic        = "market.authoritative-snapshot.v1"
	AuthoritativeSnapshotEventVersion = 1
	OptionMarketQuoteConsumerGroup    = "option-market-quote-v1"
)

// AuthoritativeSnapshotEvent is the service-neutral event emitted after an
// authoritative market snapshot has been persisted.
type AuthoritativeSnapshotEvent struct {
	Version         int32  `json:"version"`
	EventID         string `json:"event_id"`
	SnapshotID      string `json:"snapshot_id"`
	CategoryCode    string `json:"category_code"`
	Market          string `json:"market"`
	Symbol          string `json:"symbol"`
	UnderlyingPrice string `json:"underlying_price"`
	OpenPrice       string `json:"open_price"`
	HighPrice       string `json:"high_price"`
	LowPrice        string `json:"low_price"`
	Volume          string `json:"volume"`
	Turnover        string `json:"turnover"`
	QuoteTimestamp  int64  `json:"quote_timestamp"`
	PublishedAt     int64  `json:"published_at"`
}

func (e AuthoritativeSnapshotEvent) PartitionKey() string {
	return e.CategoryCode + "/" + e.Market + "/" + e.Symbol
}
