package dto

import "time"

// BinanceTradingKeyStatusDto is what a connector may read: not even the API key tail, so a separate type rather than a masked BinanceTradingKeyDto.
type BinanceTradingKeyStatusDto struct {
	Configured      bool      `json:"configured"`
	TradableMarkets []string  `json:"tradableMarkets"`
	ConfiguredAt    time.Time `json:"configuredAt"`
}
