package dto

import "time"

// BacktestResultDto is never stored, and reports the stretch actually replayed since it can
// differ from the one requested when the end falls in an unfinished interval.
type BacktestResultDto struct {
	Symbol          string             `json:"symbol"`
	Interval        string             `json:"interval"`
	StartTime       time.Time          `json:"startTime"`
	EndTime         time.Time          `json:"endTime"`
	UsedCandleCount int                `json:"usedCandleCount"`
	Summary         BacktestSummaryDto `json:"summary"`
	// ClosedTrades holds only finished round trips, earliest first, and is empty rather than
	// nil when nothing traded.
	ClosedTrades []ClosedTradeDto `json:"closedTrades"`
	EquityCurve  []EquityPointDto `json:"equityCurve"`
	// FillTiming is close or nextOpen.
	FillTiming string `json:"fillTiming"`
	// ValidationStartTime is nil when the replay was not split.
	ValidationStartTime *time.Time `json:"validationStartTime"`
	// InSample and Validation are each replayed independently from the initial capital, and
	// are nil when not split.
	InSample   *BacktestResultDto `json:"inSample,omitempty"`
	Validation *BacktestResultDto `json:"validation,omitempty"`
}
