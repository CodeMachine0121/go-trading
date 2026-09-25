package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type BacktestRequestDto struct {
	Symbol string
	// AggregationInterval is raw caller input; the domain parses it, including the empty case.
	AggregationInterval string
	// StartTime and EndTime are inclusive; a future end is read as now.
	StartTime time.Time
	EndTime   time.Time
	Script    string
	// Parameters and ParameterValues travel with the run because the replayed script may
	// never have been saved.
	Parameters      []StrategyScriptParameterWriteDto
	ParameterValues []StrategyScriptParameterValueDto
	// InitialCapital must be above zero.
	InitialCapital decimal.Decimal
	// PositionSizingValue is a percentage or an amount depending on PositionSizingMode, and
	// is ignored when staking everything.
	PositionSizingMode  string
	PositionSizingValue decimal.Decimal
	// TradingMode is carried only so a request for an unsupported mode can be refused.
	TradingMode string
	// StopLossPercentage and TakeProfitPercentage are optional percentages from entry; zero
	// means no such exit.
	StopLossPercentage   decimal.Decimal
	TakeProfitPercentage decimal.Decimal
	// Leverage of nothing, zero or one means fully paid; anything above one is carried only
	// to be refused.
	Leverage decimal.Decimal
	// MaintenanceMarginRate is carried only to be refused, since an unleveraged account
	// cannot be liquidated.
	MaintenanceMarginRate decimal.Decimal
	// EntryCostPercentage and ExitCostPercentage are optional percentages of traded value; a
	// missing exit cost defaults to the entry cost.
	EntryCostPercentage decimal.Decimal
	ExitCostPercentage  decimal.Decimal
	// FillTiming is close (blank) or nextOpen.
	FillTiming string
	// ValidationStartTime splits the replay for validation; zero is no split.
	ValidationStartTime time.Time
}
