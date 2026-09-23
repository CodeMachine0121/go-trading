package dto

import "time"

// TradingStrategyDto is one trading strategy as it is handed back: what it is made
// of, and nothing about any machine that might be following it.
//
// The two condition trees arrive nested, already reassembled from the flat rows they
// are stored as. Every reader gets them in the shape a person wrote them in.
type TradingStrategyDto struct {
	ID uint `json:"id"`
	// OwnerID is who this belongs to, and never leaves in a response — a person
	// reading their own trading strategies learns nothing from being told they are
	// theirs. It is here because a round has no signed-in caller to ask: the clock
	// started it, and resolving this trading strategy's scripts has to be done as
	// the person who owns the bot.
	OwnerID uint   `json:"-"`
	Name    string `json:"name"`
	// MarketDataKind is which kind of market its sources eat: kCandle or contractKCandle.
	MarketDataKind string `json:"marketDataKind"`
	// TradingMode is the contract trading mode of a contract trading strategy. A K
	// candle one has none, and answers without the field at all — spot has no mode.
	TradingMode   string                           `json:"tradingMode,omitempty"`
	SignalSources []TradingStrategySignalSourceDto `json:"signalSources"`
	BuyCondition  TradingStrategyConditionDto      `json:"buyCondition"`
	SellCondition TradingStrategyConditionDto      `json:"sellCondition"`
	CreatedAt     time.Time                        `json:"createdAt"`
	UpdatedAt     time.Time                        `json:"updatedAt"`
}
