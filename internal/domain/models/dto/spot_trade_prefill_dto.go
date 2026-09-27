package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// SpotTradePrefillDto is what a spot bot round's journal link fills in; nothing is recorded until the person saves.
type SpotTradePrefillDto struct {
	// Mode is newTrade, addBuyFill, addSellFill, or noOpenHolding for an exit with nothing held.
	Mode string `json:"mode"`
	// TargetTradeID names the open trade a buy or sell is added to.
	TargetTradeID   *uint     `json:"targetTradeId"`
	StrategyBotID   uint      `json:"strategyBotId"`
	StrategyBotName string    `json:"strategyBotName"`
	RunNumber       int       `json:"runNumber"`
	RanAt           time.Time `json:"ranAt"`
	// Signal is buy or sell.
	Signal string `json:"signal"`
	Symbol string `json:"symbol"`
	Market string `json:"market"`
	// Price is the round's reference price, which is not a fill; the person must replace it with their own.
	Price                  decimal.NullDecimal `json:"price"`
	Quantity               decimal.NullDecimal `json:"quantity"`
	PlannedStopLossPrice   decimal.NullDecimal `json:"plannedStopLossPrice"`
	PlannedTakeProfitPrice decimal.NullDecimal `json:"plannedTakeProfitPrice"`
	TradingStrategyID      *uint               `json:"tradingStrategyId"`
	PriceNeedsConfirmation bool                `json:"priceNeedsConfirmation"`
	// MissingReferenceReason is roundPredatesReferencePrices for rounds run before reference prices were kept.
	MissingReferenceReason string `json:"missingReferenceReason"`
}
