package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractTradeRecordRequest names no owner, since the owner comes from the token.
type ContractTradeRecordRequest struct {
	Symbol    string `json:"symbol"`
	Direction string `json:"direction"`
	// Leverage left out means one.
	Leverage          decimal.Decimal          `json:"leverage"`
	FirstEntryFill    ContractTradeFillRequest `json:"firstEntryFill"`
	Plan              ContractTradePlanRequest `json:"plan"`
	TradingStrategyID *uint                    `json:"tradingStrategyId"`
	SetupTagIDs       []uint                   `json:"setupTagIds"`
	// JournalLinkIdentifier is the round a bot message's link named, when the trade started there.
	JournalLinkIdentifier string `json:"journalLinkIdentifier"`
}

func (recordRequest ContractTradeRecordRequest) ToWriteDto() dto.ContractTradeRecordWriteDto {
	return dto.ContractTradeRecordWriteDto{
		Symbol:                recordRequest.Symbol,
		Direction:             recordRequest.Direction,
		Leverage:              recordRequest.Leverage,
		FirstEntryFill:        recordRequest.FirstEntryFill.ToWriteDto(),
		Plan:                  recordRequest.Plan.ToWriteDto(),
		TradingStrategyID:     recordRequest.TradingStrategyID,
		SetupTagIDs:           recordRequest.SetupTagIDs,
		JournalLinkIdentifier: recordRequest.JournalLinkIdentifier,
	}
}

type ContractTradeFillRequest struct {
	Kind string `json:"kind"`
	// FilledAt left out means now.
	FilledAt  *time.Time      `json:"filledAt"`
	Price     decimal.Decimal `json:"price"`
	Quantity  decimal.Decimal `json:"quantity"`
	Liquidity string          `json:"liquidity"`
	// Fee left out or null is worked out from the person's fee rate.
	Fee decimal.NullDecimal `json:"fee"`
}

func (fillRequest ContractTradeFillRequest) ToWriteDto() dto.ContractTradeFillWriteDto {
	return dto.ContractTradeFillWriteDto{
		Kind:      fillRequest.Kind,
		FilledAt:  fillRequest.FilledAt,
		Price:     fillRequest.Price,
		Quantity:  fillRequest.Quantity,
		Liquidity: fillRequest.Liquidity,
		Fee:       fillRequest.Fee,
	}
}

type ContractTradePlanRequest struct {
	PlannedStopLossPrice   decimal.NullDecimal `json:"plannedStopLossPrice"`
	PlannedTakeProfitPrice decimal.NullDecimal `json:"plannedTakeProfitPrice"`
	EntryReason            string              `json:"entryReason"`
	Confidence             *int                `json:"confidence"`
}

func (planRequest ContractTradePlanRequest) ToWriteDto() dto.ContractTradePlanWriteDto {
	return dto.ContractTradePlanWriteDto{
		PlannedStopLossPrice:   planRequest.PlannedStopLossPrice,
		PlannedTakeProfitPrice: planRequest.PlannedTakeProfitPrice,
		EntryReason:            planRequest.EntryReason,
		Confidence:             planRequest.Confidence,
	}
}

type ContractTradeNoteRequest struct {
	Content string `json:"content"`
}

type ContractTradeReviewRequest struct {
	WentWell       string `json:"wentWell"`
	WentWrong      string `json:"wentWrong"`
	NextTime       string `json:"nextTime"`
	ExecutionScore int    `json:"executionScore"`
	MistakeTagIDs  []uint `json:"mistakeTagIds"`
}

func (reviewRequest ContractTradeReviewRequest) ToWriteDto() dto.ContractTradeReviewWriteDto {
	return dto.ContractTradeReviewWriteDto{
		WentWell:       reviewRequest.WentWell,
		WentWrong:      reviewRequest.WentWrong,
		NextTime:       reviewRequest.NextTime,
		ExecutionScore: reviewRequest.ExecutionScore,
		MistakeTagIDs:  reviewRequest.MistakeTagIDs,
	}
}

type ContractTradeSetupTagsRequest struct {
	SetupTagIDs []uint `json:"setupTagIds"`
}
