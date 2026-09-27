package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type ContractTradeRecordDto struct {
	ID          uint                   `json:"id"`
	Symbol      string                 `json:"symbol"`
	Direction   string                 `json:"direction"`
	Leverage    decimal.Decimal        `json:"leverage"`
	Status      string                 `json:"status"`
	Plan        ContractTradePlanDto   `json:"plan"`
	Fills       []ContractTradeFillDto `json:"fills"`
	Notes       []ContractTradeNoteDto `json:"notes"`
	SetupTags   []TradeTagDto          `json:"setupTags"`
	MistakeTags []TradeTagDto          `json:"mistakeTags"`
	// TradingStrategyID nil means the person judged this one themselves.
	TradingStrategyID      *uint                   `json:"tradingStrategyId"`
	TradingStrategyName    string                  `json:"tradingStrategyName"`
	TradingStrategyDeleted bool                    `json:"tradingStrategyDeleted"`
	Source                 *ContractTradeSourceDto `json:"source"`
	Review                 *ContractTradeReviewDto `json:"review"`
	OpenedAt               time.Time               `json:"openedAt"`
	ClosedAt               *time.Time              `json:"closedAt"`
	AverageEntryPrice      decimal.Decimal         `json:"averageEntryPrice"`
	AverageExitPrice       decimal.NullDecimal     `json:"averageExitPrice"`
	EnteredQuantity        decimal.Decimal         `json:"enteredQuantity"`
	Position               decimal.Decimal         `json:"position"`
	Outcome                ContractTradeOutcomeDto `json:"outcome"`
}

type ContractTradePlanDto struct {
	PlannedStopLossPrice   decimal.NullDecimal `json:"plannedStopLossPrice"`
	PlannedTakeProfitPrice decimal.NullDecimal `json:"plannedTakeProfitPrice"`
	EntryReason            string              `json:"entryReason"`
	Confidence             *int                `json:"confidence"`
	// Locked is true once the trade is closed.
	Locked bool `json:"locked"`
}

type ContractTradeFillDto struct {
	ID        uint            `json:"id"`
	Kind      string          `json:"kind"`
	FilledAt  time.Time       `json:"filledAt"`
	Price     decimal.Decimal `json:"price"`
	Quantity  decimal.Decimal `json:"quantity"`
	Liquidity string          `json:"liquidity"`
	Fee       decimal.Decimal `json:"fee"`
	// FeeRateMissing marks a zero fee that only means no rate was set.
	FeeRateMissing bool `json:"feeRateMissing"`
}

type ContractTradeNoteDto struct {
	ID        uint      `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// ContractTradeSourceDto is the bot round's suggestion as it was, kept because the bot forgets old rounds.
type ContractTradeSourceDto struct {
	StrategyBotID            uint                `json:"strategyBotId"`
	StrategyBotName          string              `json:"strategyBotName"`
	RunNumber                int                 `json:"runNumber"`
	ReferencePrice           decimal.NullDecimal `json:"referencePrice"`
	SuggestedStopLossPrice   decimal.NullDecimal `json:"suggestedStopLossPrice"`
	SuggestedTakeProfitPrice decimal.NullDecimal `json:"suggestedTakeProfitPrice"`
}

type ContractTradeReviewDto struct {
	WentWell       string    `json:"wentWell"`
	WentWrong      string    `json:"wentWrong"`
	NextTime       string    `json:"nextTime"`
	ExecutionScore int       `json:"executionScore"`
	ReviewedAt     time.Time `json:"reviewedAt"`
}

type ContractTradeRecordPageDto struct {
	Trades     []ContractTradeRecordDto `json:"trades"`
	TotalCount int64                    `json:"totalCount"`
}
