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
	// TradingMode is what the caller declared about which rules to trade by, exactly
	// as they typed it. There is one set of rules left, so nothing is chosen by it —
	// it is carried only so that a caller asking for a different one is told, rather
	// than handed a report card of a run they did not ask for.
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
	// Leverage is what the caller declared about borrowing. Nothing, zero and one all
	// describe a position paid for in full, which is the only kind this system
	// replays; anything above one is carried here only so that it can be refused.
	Leverage decimal.Decimal
	// MaintenanceMarginRate is what the caller declared about being closed out for
	// running low on collateral. Only an account that borrowed can be, so anything
	// other than nothing is carried here only so that it can be refused.
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
	// FillTiming is at what price this replay fills its signals: close (blank) or
	// nextOpen.
	FillTiming string
	// ValidationStartTime splits the replay for validation; zero is no split.
	ValidationStartTime time.Time
}
