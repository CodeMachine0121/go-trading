package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// TradingStrategyBacktestRequestDto is the shape the application hands the domain to
// replay one trading strategy over one stretch of market.
//
// The coarseness is **not** here. A trading strategy's signal sources each carry
// their own, and this version replays only the ones that agree — so asking the caller
// for it as well would be asking them to repeat something the trading strategy
// already says, with no way to tell which answer wins.
type TradingStrategyBacktestRequestDto struct {
	Symbol string
	// StartTime and EndTime bound the stretch to replay, both ends included. An end
	// that has not arrived yet is read as now.
	StartTime time.Time
	EndTime   time.Time
	// SignalSources arrive already resolved: the application has walked each named
	// strategy script through its gates and brought back the script itself.
	SignalSources []ResolvedSignalSourceDto
	BuyCondition  TradingStrategyConditionDto
	SellCondition TradingStrategyConditionDto
	// InitialCapital is what the account starts with. It must be above zero: with no
	// capital there is nothing to stake.
	InitialCapital decimal.Decimal
	// PositionSizingMode is how much each opening stakes, exactly as declared, and
	// PositionSizingValue the figure that goes with it.
	PositionSizingMode  string
	PositionSizingValue decimal.Decimal
	// TradingMode is which set of rules the replay trades by, exactly as declared.
	TradingMode string
}

// ResolvedSignalSourceDto is one signal source with the script it names already
// fetched — the one thing a replay cannot get for itself, because whether a caller
// may read a strategy script is a question about people, not about candles.
type ResolvedSignalSourceDto struct {
	Label               string
	AggregationInterval string
	Script              string
	// Parameters are the script's knobs as it declares them, and ParameterValues what
	// this source sets them to.
	Parameters      []StrategyScriptParameterWriteDto
	ParameterValues []StrategyScriptParameterValueDto
}
