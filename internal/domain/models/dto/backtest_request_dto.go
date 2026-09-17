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
}
