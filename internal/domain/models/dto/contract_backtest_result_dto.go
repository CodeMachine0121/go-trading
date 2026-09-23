package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractBacktestResultDto is everything one contract replay produced. Nothing of it
// is stored.
type ContractBacktestResultDto struct {
	Symbol          string                     `json:"symbol"`
	Interval        string                     `json:"interval"`
	TradingMode     string                     `json:"tradingMode"`
	Leverage        decimal.Decimal            `json:"leverage"`
	StartTime       time.Time                  `json:"startTime"`
	EndTime         time.Time                  `json:"endTime"`
	UsedCandleCount int                        `json:"usedCandleCount"`
	Summary         ContractBacktestSummaryDto `json:"summary"`
	ClosedTrades    []ContractClosedTradeDto   `json:"closedTrades"`
	EquityCurve     []EquityPointDto           `json:"equityCurve"`
	// FillTiming is at what price this replay filled its signals: close or nextOpen.
	FillTiming string `json:"fillTiming"`
	// ValidationStartTime is where this replay was split, or absent when it was not.
	ValidationStartTime *time.Time `json:"validationStartTime"`
	// InSample and Validation are the two parts of a split replay, each replayed on
	// its own from the initial capital and flat. Both are absent when it was not split.
	InSample   *ContractBacktestResultDto `json:"inSample,omitempty"`
	Validation *ContractBacktestResultDto `json:"validation,omitempty"`
}
