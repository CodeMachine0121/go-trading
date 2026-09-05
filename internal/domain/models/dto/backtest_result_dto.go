package dto

import "time"

// BacktestResultDto is the only shape a replay leaves the domain in, and it is never
// stored: asking the same question twice runs the replay twice.
//
// It says which stretch of market it actually read, and that is not a courtesy. What
// was asked for and what was replayed differ whenever the requested end reaches into
// an interval that has not finished, and a caller drawing a curve has to know which
// of the two it is looking at.
type BacktestResultDto struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
	// StartTime and EndTime are where the candles actually replayed begin and end.
	StartTime       time.Time          `json:"startTime"`
	EndTime         time.Time          `json:"endTime"`
	UsedCandleCount int                `json:"usedCandleCount"`
	Summary         BacktestSummaryDto `json:"summary"`
	// ClosedTrades holds only round trips that finished, earliest first. It is empty
	// rather than absent when a strategy never traded, which is a legitimate answer.
	ClosedTrades []ClosedTradeDto `json:"closedTrades"`
	// EquityCurve holds one point per replayed candle, earliest first.
	EquityCurve []EquityPointDto `json:"equityCurve"`
}
