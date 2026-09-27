package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// SpotTradeRecordRequest names no owner, since the owner comes from the token.
type SpotTradeRecordRequest struct {
	Symbol       string               `json:"symbol"`
	FirstBuyFill SpotTradeFillRequest `json:"firstBuyFill"`
	Plan         SpotTradePlanRequest `json:"plan"`
	// Leverage and Direction are read only so a spot trade that names either can be refused.
	Leverage          decimal.NullDecimal `json:"leverage"`
	Direction         string              `json:"direction"`
	TradingStrategyID *uint               `json:"tradingStrategyId"`
	SetupTagIDs       []uint              `json:"setupTagIds"`
	// JournalLinkIdentifier is the round a bot message's link named, when the trade started there.
	JournalLinkIdentifier string `json:"journalLinkIdentifier"`
}

func (recordRequest SpotTradeRecordRequest) ToWriteDto() dto.SpotTradeRecordWriteDto {
	return dto.SpotTradeRecordWriteDto{
		Symbol:                recordRequest.Symbol,
		FirstBuyFill:          recordRequest.FirstBuyFill.ToWriteDto(),
		Plan:                  recordRequest.Plan.ToWriteDto(),
		Leverage:              recordRequest.Leverage,
		Direction:             recordRequest.Direction,
		TradingStrategyID:     recordRequest.TradingStrategyID,
		SetupTagIDs:           recordRequest.SetupTagIDs,
		JournalLinkIdentifier: recordRequest.JournalLinkIdentifier,
	}
}

type SpotTradeFillRequest struct {
	Kind string `json:"kind"`
	// FilledAt left out means now.
	FilledAt *time.Time      `json:"filledAt"`
	Price    decimal.Decimal `json:"price"`
	Quantity decimal.Decimal `json:"quantity"`
	// Fee left out or null is zero.
	Fee decimal.NullDecimal `json:"fee"`
}

func (fillRequest SpotTradeFillRequest) ToWriteDto() dto.SpotTradeFillWriteDto {
	return dto.SpotTradeFillWriteDto{
		Kind:     fillRequest.Kind,
		FilledAt: fillRequest.FilledAt,
		Price:    fillRequest.Price,
		Quantity: fillRequest.Quantity,
		Fee:      fillRequest.Fee,
	}
}

type SpotTradePlanRequest struct {
	PlannedStopLossPrice   decimal.NullDecimal `json:"plannedStopLossPrice"`
	PlannedTakeProfitPrice decimal.NullDecimal `json:"plannedTakeProfitPrice"`
	EntryReason            string              `json:"entryReason"`
	Confidence             *int                `json:"confidence"`
}

func (planRequest SpotTradePlanRequest) ToWriteDto() dto.SpotTradePlanWriteDto {
	return dto.SpotTradePlanWriteDto{
		PlannedStopLossPrice:   planRequest.PlannedStopLossPrice,
		PlannedTakeProfitPrice: planRequest.PlannedTakeProfitPrice,
		EntryReason:            planRequest.EntryReason,
		Confidence:             planRequest.Confidence,
	}
}

type SpotTradeNoteRequest struct {
	Content string `json:"content"`
}

type SpotTradeReviewRequest struct {
	WentWell       string `json:"wentWell"`
	WentWrong      string `json:"wentWrong"`
	NextTime       string `json:"nextTime"`
	ExecutionScore int    `json:"executionScore"`
	MistakeTagIDs  []uint `json:"mistakeTagIds"`
}

func (reviewRequest SpotTradeReviewRequest) ToWriteDto() dto.SpotTradeReviewWriteDto {
	return dto.SpotTradeReviewWriteDto{
		WentWell:       reviewRequest.WentWell,
		WentWrong:      reviewRequest.WentWrong,
		NextTime:       reviewRequest.NextTime,
		ExecutionScore: reviewRequest.ExecutionScore,
		MistakeTagIDs:  reviewRequest.MistakeTagIDs,
	}
}

type SpotTradeSetupTagsRequest struct {
	SetupTagIDs []uint `json:"setupTagIds"`
}
