package cache

// SettlementSnapshot is the immutable price input consumed by financial
// settlement. SnapshotID and Revision bind a calculation to exact source data.
type SettlementSnapshot struct {
	SnapshotID        string `json:"snapshotId"`
	Kind              string `json:"kind"`
	CategoryCode      string `json:"categoryCode"`
	Market            string `json:"market"`
	Symbol            string `json:"symbol"`
	Price             string `json:"price,omitempty"`
	MarkPrice         string `json:"markPrice,omitempty"`
	IndexPrice        string `json:"indexPrice,omitempty"`
	FundingRate       string `json:"fundingRate,omitempty"`
	Source            string `json:"source"`
	SourceTimestamp   int64  `json:"sourceTimestamp"`
	SnapshotTimestamp int64  `json:"snapshotTimestamp"`
	Revision          int64  `json:"revision"`
	FormulaVersion    string `json:"formulaVersion,omitempty"`
	Authority         string `json:"authority,omitempty"`
	Confirmed         bool   `json:"confirmed"`
}

type DepthLevel struct {
	Price        float64 `json:"p"`
	Volume       float64 `json:"v"`
	Position     int64   `json:"po"`
	OriginVolume float64 `json:"o"`
}
type DepthPayload struct {
	Asks []*DepthLevel `json:"asks"`
	Bids []*DepthLevel `json:"bids"`
}
type QuotePayload struct {
	LastPrice float64 `json:"lastPrice"`
	// LastPriceText preserves the exact decimal token received from the source.
	// Financial settlement must use this field instead of converting LastPrice.
	LastPriceText string  `json:"lastPriceText,omitempty"`
	Open          float64 `json:"open"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	Volume        float64 `json:"volume"`
	Turnover      float64 `json:"turnover"`
	Ts            int64   `json:"ts"`
	Authority     string  `json:"authority,omitempty"`
}
type TickPayload struct {
	LastPrice float64 `json:"lastPrice"`
	Volume    float64 `json:"volume"`
	Turnover  float64 `json:"turnover"`
	Ts        int64   `json:"ts"`
}
type KlinePayload struct {
	Interval      string  `json:"interval"`
	Open          float64 `json:"open"`
	High          float64 `json:"high"`
	Low           float64 `json:"low"`
	Close         float64 `json:"close"`
	Volume        float64 `json:"volume"`
	Turnover      float64 `json:"turnover"`
	Ts            int64   `json:"ts"`
	Source        string  `json:"source,omitempty"`
	Revision      int64   `json:"revision,omitempty"`
	IsClosed      bool    `json:"isClosed"`
	Confirmed     bool    `json:"confirmed"`
	ActualCount   int32   `json:"actualCount,omitempty"`
	ExpectedCount int32   `json:"expectedCount,omitempty"`
}
type Topic string

const (
	TopicQuote Topic = "quote"
	TopicDepth Topic = "depth"
	TopicTick  Topic = "tick"
	TopicKline Topic = "kline"
)

type ClientMessage struct {
	Topic        Topic  `json:"topic"`
	CategoryCode string `json:"categoryCode"`
	Symbol       string `json:"symbol"`
	Market       string `json:"market,omitempty"`
	Interval     string `json:"interval,omitempty"`
}
