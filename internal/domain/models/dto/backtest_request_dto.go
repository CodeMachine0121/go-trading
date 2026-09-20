package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// BacktestRequestDto is the shape the application hands the domain to replay one
// strategy script over one stretch of market.
//
// How coarse the candles are and which stretch to read are here rather than on the
// strategy script, for the same reason an indicator calculation carries them: they describe
// one run, so the same algorithm can be replayed at any coarseness over any stretch.
type BacktestRequestDto struct {
	Symbol string
	// AggregationInterval is the coarseness the caller declared, exactly as written.
	// Reading it — including leaving it out — is the domain's job.
	AggregationInterval string
	// StartTime and EndTime bound the stretch to replay, both ends included. An end
	// that has not arrived yet is read as now.
	StartTime time.Time
	EndTime   time.Time
	Script    string
	// Parameters are the algorithm's knobs as declared, and ParameterValues what they
	// are worth this time. Both arrive with the run, because what is replayed is a
	// script that may never have been saved.
	Parameters      []StrategyScriptParameterWriteDto
	ParameterValues []StrategyScriptParameterValueDto
	// InitialCapital is what the account starts with. It must be above zero: with no
	// capital there is nothing to stake.
	InitialCapital decimal.Decimal
	// PositionSizingMode is how much each opening stakes, exactly as declared, and
	// PositionSizingValue the figure that goes with it — a percentage or an amount.
	// Staking everything needs no figure and ignores it.
	PositionSizingMode  string
	PositionSizingValue decimal.Decimal
	// TradingMode is which set of rules the replay trades by, exactly as declared.
	// Reading it — including leaving it out — is the domain's job.
	TradingMode string
	// StopLossPercentage and TakeProfitPercentage are how far from its entry a
	// position may be wrong, and how far right is far enough. Both are percentages,
	// both optional, and zero means there is no such exit.
	//
	// They travel with the run rather than with the strategy script, for the same
	// reason the capital does: they are what somebody turns up and down while
	// sitting there deciding what they can sit through.
	StopLossPercentage   decimal.Decimal
	TakeProfitPercentage decimal.Decimal
	// Leverage is how many times the stake this run's positions are exposed to, and
	// MaintenanceMarginRate how little of that exposure may be left before the loan
	// is called in. Both are optional; leverage of nothing, zero or one means
	// nothing is borrowed and no forced exit is simulated.
	//
	// They travel with the run for the same reason the capital and the exit distances
	// do: a strategy script has no opinion about how much its owner is willing to
	// borrow, and the same script is worth replaying against more than one answer.
	Leverage              decimal.Decimal
	MaintenanceMarginRate decimal.Decimal
	// EntryCostPercentage and ExitCostPercentage are what the act of trading costs at
	// each end, as a percentage of the money that changes hands. Both are optional
	// and zero means no charge on that side — except that leaving the exit out is
	// read as "the same as the entry", because the two are the halves of one thing
	// rather than two separate things.
	//
	// They travel with the run for the same reason the capital and the exit distances
	// do: a strategy script has no opinion about what a broker charges, and the same
	// script is worth replaying against more than one answer.
	EntryCostPercentage decimal.Decimal
	ExitCostPercentage  decimal.Decimal
}
