package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractBacktestResultDto is never stored.
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
	// FillTiming is close or nextOpen.
	FillTiming string `json:"fillTiming"`
	// ValidationStartTime is nil when the replay was not split.
	ValidationStartTime *time.Time `json:"validationStartTime"`
	// InSample and Validation are each replayed independently from the initial capital, and
	// are nil when not split.
	InSample   *ContractBacktestResultDto `json:"inSample,omitempty"`
	Validation *ContractBacktestResultDto `json:"validation,omitempty"`
}
