package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// TradingStrategyBacktestRequest takes the strategy from the path and carries no interval or script, since the strategy already defines both.
type TradingStrategyBacktestRequest struct {
	Symbol string `json:"symbol"`
	// Carried only to be refused; an undeclared field would be silently dropped and yield a spot report.
	TradingMode    string          `json:"tradingMode"`
	StartTime      time.Time       `json:"startTime"`
	EndTime        time.Time       `json:"endTime"`
	InitialCapital decimal.Decimal `json:"initialCapital"`
	// Staking everything needs no PositionSizingValue.
	PositionSizingMode  string          `json:"positionSizingMode"`
	PositionSizingValue decimal.Decimal `json:"positionSizingValue"`
	// Asked for per run because the strategy has no opinion on exit distances.
	StopLossPercentage   decimal.Decimal `json:"stopLossPercentage"`
	TakeProfitPercentage decimal.Decimal `json:"takeProfitPercentage"`
	// Only carried to refuse borrowing: empty, zero and one mean fully paid, anything above one is refused.
	Leverage decimal.Decimal `json:"leverage"`
	// Carried only to be refused; an undeclared field would be silently dropped.
	MaintenanceMarginRate decimal.Decimal `json:"maintenanceMarginRate"`
	// Asked for per run because trading costs depend on the owner's broker, not the strategy.
	EntryCostPercentage decimal.Decimal `json:"entryCostPercentage"`
	ExitCostPercentage  decimal.Decimal `json:"exitCostPercentage"`
	// FillTiming is close (default, the signalling bar's close) or nextOpen (the next bar's open).
	FillTiming string `json:"fillTiming"`
	// ValidationStartTime, when given, splits the replay into in-sample and validation parts, each starting from the initial capital.
	ValidationStartTime time.Time `json:"validationStartTime"`
}

// ToRequestDto omits signal sources, conditions and trading mode, which come from the strategy named by the path.
func (request TradingStrategyBacktestRequest) ToRequestDto() dto.TradingStrategyBacktestRequestDto {
	return dto.TradingStrategyBacktestRequestDto{
		Symbol:               request.Symbol,
		TradingMode:          request.TradingMode,
		StartTime:            request.StartTime,
		EndTime:              request.EndTime,
		InitialCapital:       request.InitialCapital,
		PositionSizingMode:   request.PositionSizingMode,
		PositionSizingValue:  request.PositionSizingValue,
		StopLossPercentage:   request.StopLossPercentage,
		TakeProfitPercentage: request.TakeProfitPercentage,

		Leverage:              request.Leverage,
		MaintenanceMarginRate: request.MaintenanceMarginRate,

		EntryCostPercentage: request.EntryCostPercentage,
		ExitCostPercentage:  request.ExitCostPercentage,
		FillTiming:          request.FillTiming,
		ValidationStartTime: request.ValidationStartTime,
	}
}
