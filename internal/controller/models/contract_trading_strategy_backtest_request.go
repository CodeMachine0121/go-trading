package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractTradingStrategyBacktestRequest is the body a caller sends to replay one of
// their contract trading strategies on a contract account. Which trading strategy is
// meant comes from the path; its sources, conditions and trading mode come from the
// trading strategy itself.
type ContractTradingStrategyBacktestRequest struct {
	Symbol               string          `json:"symbol"`
	StartTime            time.Time       `json:"startTime"`
	EndTime              time.Time       `json:"endTime"`
	InitialCapital       decimal.Decimal `json:"initialCapital"`
	PositionSizingMode   string          `json:"positionSizingMode"`
	PositionSizingValue  decimal.Decimal `json:"positionSizingValue"`
	StopLossPercentage   decimal.Decimal `json:"stopLossPercentage"`
	TakeProfitPercentage decimal.Decimal `json:"takeProfitPercentage"`
	EntryCostPercentage  decimal.Decimal `json:"entryCostPercentage"`
	ExitCostPercentage   decimal.Decimal `json:"exitCostPercentage"`
	// FillTiming is close (the default: fill at the signalling bar's close) or
	// nextOpen (fill at the next bar's open).
	FillTiming string `json:"fillTiming"`
	// ValidationStartTime, when given, splits the replay into an in-sample part and a
	// validation part, each replayed on its own from the initial capital.
	ValidationStartTime time.Time       `json:"validationStartTime"`
	Leverage            decimal.Decimal `json:"leverage"`
	SlippagePercentage  decimal.Decimal `json:"slippagePercentage"`
	// TradingMode and MaintenanceMarginRate are carried only to be refused: the
	// trading strategy says the first, the symbol's ladder the second.
	TradingMode           string          `json:"tradingMode"`
	MaintenanceMarginRate decimal.Decimal `json:"maintenanceMarginRate"`
}

func (request ContractTradingStrategyBacktestRequest) ToRequestDto() dto.ContractTradingStrategyBacktestRequestDto {
	return dto.ContractTradingStrategyBacktestRequestDto{
		Symbol:                request.Symbol,
		StartTime:             request.StartTime,
		EndTime:               request.EndTime,
		InitialCapital:        request.InitialCapital,
		PositionSizingMode:    request.PositionSizingMode,
		PositionSizingValue:   request.PositionSizingValue,
		StopLossPercentage:    request.StopLossPercentage,
		TakeProfitPercentage:  request.TakeProfitPercentage,
		EntryCostPercentage:   request.EntryCostPercentage,
		ExitCostPercentage:    request.ExitCostPercentage,
		FillTiming:            request.FillTiming,
		ValidationStartTime:   request.ValidationStartTime,
		Leverage:              request.Leverage,
		SlippagePercentage:    request.SlippagePercentage,
		TradingMode:           request.TradingMode,
		MaintenanceMarginRate: request.MaintenanceMarginRate,
	}
}
