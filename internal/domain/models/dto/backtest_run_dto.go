package dto

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

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

type BacktestRunDetailDto struct {
	BacktestRunDto
	Result *BacktestRunResultDto `json:"result"`
}

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

type BacktestRunListQueryDto struct {
	Status            string
	StrategyScriptID  uint
	TradingStrategyID uint
	Limit             int
}

type BacktestRunSummaryDto struct {
	TotalReturnRate float64
	MaximumDrawdown float64
	TradeCount      int
}
