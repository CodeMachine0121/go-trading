package dto

import "time"

// BinanceTradingKeyDto has no field for the secret key or the whole API key, so no route can leak either; Configured false is a normal state.
type BinanceTradingKeyDto struct {
	Configured      bool      `json:"configured"`
	ApiKeyTail      string    `json:"apiKeyTail"`
	TradableMarkets []string  `json:"tradableMarkets"`
	ConfiguredAt    time.Time `json:"configuredAt"`
}
