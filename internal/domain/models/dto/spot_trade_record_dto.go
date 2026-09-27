package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type SpotTradeRecordDto struct {
	ID          uint               `json:"id"`
	Symbol      string             `json:"symbol"`
	Market      string             `json:"market"`
	Currency    string             `json:"currency"`
	Status      string             `json:"status"`
	Plan        SpotTradePlanDto   `json:"plan"`
	Fills       []SpotTradeFillDto `json:"fills"`
	Notes       []SpotTradeNoteDto `json:"notes"`
	SetupTags   []TradeTagDto      `json:"setupTags"`
	MistakeTags []TradeTagDto      `json:"mistakeTags"`
	// TradingStrategyID nil means the person judged this one themselves.
	TradingStrategyID      *uint               `json:"tradingStrategyId"`
	TradingStrategyName    string              `json:"tradingStrategyName"`
	TradingStrategyDeleted bool                `json:"tradingStrategyDeleted"`
	Source                 *SpotTradeSourceDto `json:"source"`
	Review                 *SpotTradeReviewDto `json:"review"`
	OpenedAt               time.Time           `json:"openedAt"`
	ClosedAt               *time.Time          `json:"closedAt"`
	AverageBuyPrice        decimal.Decimal     `json:"averageBuyPrice"`
	AverageSellPrice       decimal.NullDecimal `json:"averageSellPrice"`
	BoughtQuantity         decimal.Decimal     `json:"boughtQuantity"`
	Holding                decimal.Decimal     `json:"holding"`
	Outcome                SpotTradeOutcomeDto `json:"outcome"`
}

type SpotTradePlanDto struct {
	PlannedStopLossPrice   decimal.NullDecimal `json:"plannedStopLossPrice"`
	PlannedTakeProfitPrice decimal.NullDecimal `json:"plannedTakeProfitPrice"`
	EntryReason            string              `json:"entryReason"`
	Confidence             *int                `json:"confidence"`
	// Locked is true once the trade is closed.
	Locked bool `json:"locked"`
}

type SpotTradeFillDto struct {
	ID       uint            `json:"id"`
	Kind     string          `json:"kind"`
	FilledAt time.Time       `json:"filledAt"`
	Price    decimal.Decimal `json:"price"`
	Quantity decimal.Decimal `json:"quantity"`
	Fee      decimal.Decimal `json:"fee"`
}

type SpotTradeNoteDto struct {
	ID        uint      `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// SpotTradeSourceDto is the bot round's suggestion as it was, kept because the bot forgets old rounds.
type SpotTradeSourceDto struct {
	StrategyBotID            uint                `json:"strategyBotId"`
	StrategyBotName          string              `json:"strategyBotName"`
	RunNumber                int                 `json:"runNumber"`
	ReferencePrice           decimal.NullDecimal `json:"referencePrice"`
	SuggestedStopLossPrice   decimal.NullDecimal `json:"suggestedStopLossPrice"`
	SuggestedTakeProfitPrice decimal.NullDecimal `json:"suggestedTakeProfitPrice"`
}

type SpotTradeReviewDto struct {
	WentWell       string    `json:"wentWell"`
	WentWrong      string    `json:"wentWrong"`
	NextTime       string    `json:"nextTime"`
	ExecutionScore int       `json:"executionScore"`
	ReviewedAt     time.Time `json:"reviewedAt"`
}

type SpotTradeRecordPageDto struct {
	Trades     []SpotTradeRecordDto `json:"trades"`
	TotalCount int64                `json:"totalCount"`
}

// SpotTradeOutcomeDto pairs every figure with whether it could be worked out; spot has no funding and cannot be liquidated.
type SpotTradeOutcomeDto struct {
	GrossProfit decimal.Decimal `json:"grossProfit"`
	TotalFee    decimal.Decimal `json:"totalFee"`
	NetProfit   decimal.Decimal `json:"netProfit"`
	BuyCost     decimal.Decimal `json:"buyCost"`
	// ReturnRate is a fraction of the buy cost.
	ReturnRate  *float64            `json:"returnRate"`
	PlannedRisk decimal.NullDecimal `json:"plannedRisk"`
	RMultiple   *float64            `json:"rMultiple"`
	// RMultipleUnavailableReason is noStopLoss when no planned stop was given.
	RMultipleUnavailableReason string            `json:"rMultipleUnavailableReason"`
	Excursion                  TradeExcursionDto `json:"excursion"`
	ProfitCaptureRate          *float64          `json:"profitCaptureRate"`
	FloatingProfit             TradeFloatingDto  `json:"floatingProfit"`
	// EntrySlippagePercentage is positive when the buy was dearer than the bot's reference price.
	EntrySlippagePercentage *float64 `json:"entrySlippagePercentage"`
}
