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
	// StopLossPercentage and TakeProfitPercentage are the two exit distances this
	// run simulates, both optional, zero meaning no such exit.
	StopLossPercentage   decimal.Decimal
	TakeProfitPercentage decimal.Decimal
	// EntryCostPercentage and ExitCostPercentage are what the act of trading costs at
	// each end, both optional. Leaving the exit out is read as "the same as the
	// entry".
	EntryCostPercentage decimal.Decimal
	ExitCostPercentage  decimal.Decimal
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

// ToBacktestRequestDto is this replay stated as the conditions every replay shares,
// once the coarseness its signal sources agree on has been worked out.
//
// It lives here rather than where the narrower shape is built because the fields are
// all this one's, and reading them from outside is how one gets left behind: a
// condition added to a replay and forgotten here would not fail to compile, it would
// quietly read as nothing at all — no capital, no sizing mode, the default way of
// trading — and the replay would run anyway.
//
// It carries no script and no knobs. Those belong to each signal source separately,
// and a replay of a whole trading strategy runs one script per source rather than one
// for the lot.
func (requestDto TradingStrategyBacktestRequestDto) ToBacktestRequestDto(
	sharedAggregationInterval string,
) BacktestRequestDto {
	return BacktestRequestDto{
		Symbol:               requestDto.Symbol,
		AggregationInterval:  sharedAggregationInterval,
		StartTime:            requestDto.StartTime,
		EndTime:              requestDto.EndTime,
		InitialCapital:       requestDto.InitialCapital,
		PositionSizingMode:   requestDto.PositionSizingMode,
		PositionSizingValue:  requestDto.PositionSizingValue,
		TradingMode:          requestDto.TradingMode,
		StopLossPercentage:   requestDto.StopLossPercentage,
		TakeProfitPercentage: requestDto.TakeProfitPercentage,
		EntryCostPercentage:  requestDto.EntryCostPercentage,
		ExitCostPercentage:   requestDto.ExitCostPercentage,
	}
}
