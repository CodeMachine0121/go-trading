package dto

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// BacktestRunDto is one run as a list shows it: what it replays, where it has got to, and the headline figures once done; never the report itself.
type BacktestRunDto struct {
	ID              uint      `json:"id"`
	Kind            string    `json:"kind"`
	Status          string    `json:"status"`
	ProgressPercent int       `json:"progressPercent"`
	Symbol          string    `json:"symbol"`
	Interval        string    `json:"interval"`
	StartTime       time.Time `json:"startTime"`
	EndTime         time.Time `json:"endTime"`
	// Leverage and TradingMode are nil for spot replays.
	Leverage          *decimal.Decimal `json:"leverage"`
	TradingMode       *string          `json:"tradingMode"`
	StrategyScriptID  *uint            `json:"strategyScriptId"`
	TradingStrategyID *uint            `json:"tradingStrategyId"`
	SubjectName       string           `json:"subjectName"`
	SubmittedAt       time.Time        `json:"submittedAt"`
	StartedAt         *time.Time       `json:"startedAt"`
	FinishedAt        *time.Time       `json:"finishedAt"`
	// TotalReturnRate, MaximumDrawdown and TradeCount are nil until the run succeeds.
	TotalReturnRate *float64 `json:"totalReturnRate"`
	MaximumDrawdown *float64 `json:"maximumDrawdown"`
	TradeCount      *int     `json:"tradeCount"`
	FailureReason   string   `json:"failureReason"`
}

// BacktestRunDetailDto is a run with its report, which is nil until the run succeeds.
type BacktestRunDetailDto struct {
	BacktestRunDto
	Result *BacktestRunResultDto `json:"result"`
}

// BacktestRunResultDto carries whichever report the run's kind produces, each exactly as the on-demand replay answers it.
type BacktestRunResultDto struct {
	Spot     *BacktestResultDto
	Contract *ContractBacktestResultDto
}

// MarshalJSON writes the one report present as itself, so a run's result reads exactly like the on-demand answer.
func (resultDto BacktestRunResultDto) MarshalJSON() ([]byte, error) {
	if resultDto.Contract != nil {
		return json.Marshal(resultDto.Contract)
	}
	if resultDto.Spot != nil {
		return json.Marshal(resultDto.Spot)
	}

	return []byte("null"), nil
}

type BacktestRunPageDto struct {
	Runs       []BacktestRunDto `json:"runs"`
	TotalCount int64            `json:"totalCount"`
}

// BacktestRunListQueryDto is read leniently: an unreadable limit falls back to the default rather than failing the list.
type BacktestRunListQueryDto struct {
	Status            string
	StrategyScriptID  uint
	TradingStrategyID uint
	Limit             int
}

// BacktestRunSummaryDto is the handful of figures a finished run keeps on itself for lists.
type BacktestRunSummaryDto struct {
	TotalReturnRate float64
	MaximumDrawdown float64
	TradeCount      int
}
