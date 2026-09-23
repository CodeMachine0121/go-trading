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
	// TradingStrategyMarketDataKind is the kind of market the trading strategy being
	// replayed is written for, so that one written for contracts is refused here.
	TradingStrategyMarketDataKind string
	BuyCondition                  TradingStrategyConditionDto
	SellCondition                 TradingStrategyConditionDto
	// InitialCapital is what the account starts with. It must be above zero: with no
	// capital there is nothing to stake.
	InitialCapital decimal.Decimal
	// PositionSizingMode is how much each opening stakes, exactly as declared, and
	// PositionSizingValue the figure that goes with it.
	PositionSizingMode  string
	PositionSizingValue decimal.Decimal
	// StopLossPercentage and TakeProfitPercentage are the two exit distances this
	// run simulates, both optional, zero meaning no such exit.
	StopLossPercentage   decimal.Decimal
	TakeProfitPercentage decimal.Decimal
	// TradingMode is what the caller declared about which rules to trade by, exactly
	// as they typed it. There is one set of rules left, so nothing is chosen by it —
	// it is carried only so that a caller asking for a different one is told, rather
	// than handed a report card of a run they did not ask for.
	TradingMode string
	// Leverage is what the caller declared about borrowing. Nothing, zero and one all
	// describe a position paid for in full, which is the only kind this system
	// replays; anything above one is carried here only so that it can be refused.
	Leverage decimal.Decimal
	// MaintenanceMarginRate is what the caller declared about being closed out for
	// running low on collateral, carried for the reason Leverage is: only a borrowed
	// position can be, so anything other than nothing exists here to be refused.
	MaintenanceMarginRate decimal.Decimal
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
	// MarketDataKind is the kind of market the named strategy script eats.
	MarketDataKind string
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

		Leverage:              requestDto.Leverage,
		MaintenanceMarginRate: requestDto.MaintenanceMarginRate,

		EntryCostPercentage: requestDto.EntryCostPercentage,
		ExitCostPercentage:  requestDto.ExitCostPercentage,
	}
}
