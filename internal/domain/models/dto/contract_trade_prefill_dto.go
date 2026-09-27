package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractTradePrefillDto is what a bot round's journal link fills in; nothing is recorded until the person saves.
type ContractTradePrefillDto struct {
	// Mode is newTrade, or addEntryFill when the person already holds this symbol in this direction.
	Mode string `json:"mode"`
	// TargetTradeID names the open trade an addEntryFill goes to.
	TargetTradeID          *uint               `json:"targetTradeId"`
	StrategyBotID          uint                `json:"strategyBotId"`
	StrategyBotName        string              `json:"strategyBotName"`
	RunNumber              int                 `json:"runNumber"`
	RanAt                  time.Time           `json:"ranAt"`
	Symbol                 string              `json:"symbol"`
	Direction              string              `json:"direction"`
	Leverage               decimal.NullDecimal `json:"leverage"`
	PlannedStopLossPrice   decimal.NullDecimal `json:"plannedStopLossPrice"`
	PlannedTakeProfitPrice decimal.NullDecimal `json:"plannedTakeProfitPrice"`
	TradingStrategyID      *uint               `json:"tradingStrategyId"`
	// EntryPrice is the round's reference price, which is not a fill; the person must replace it with their own.
	EntryPrice                  decimal.NullDecimal `json:"entryPrice"`
	Quantity                    decimal.NullDecimal `json:"quantity"`
	EntryPriceNeedsConfirmation bool                `json:"entryPriceNeedsConfirmation"`
	// MissingReferenceReason is roundPredatesReferencePrices for rounds run before reference prices were kept.
	MissingReferenceReason string `json:"missingReferenceReason"`
}
