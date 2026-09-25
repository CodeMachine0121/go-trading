package dto

import "time"

// TradingStrategyDto returns condition trees nested, reassembled from their stored flat rows.
type TradingStrategyDto struct {
	ID uint `json:"id"`
	// OwnerID is never serialized; rounds use it to resolve scripts as the bot's owner.
	OwnerID uint   `json:"-"`
	Name    string `json:"name"`
	// MarketDataKind is kCandle or contractKCandle.
	MarketDataKind string `json:"marketDataKind"`
	// TradingMode is omitted for spot trading strategies.
	TradingMode   string                           `json:"tradingMode,omitempty"`
	SignalSources []TradingStrategySignalSourceDto `json:"signalSources"`
	BuyCondition  TradingStrategyConditionDto      `json:"buyCondition"`
	SellCondition TradingStrategyConditionDto      `json:"sellCondition"`
	CreatedAt     time.Time                        `json:"createdAt"`
	UpdatedAt     time.Time                        `json:"updatedAt"`
}
