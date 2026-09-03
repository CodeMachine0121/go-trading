package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// IndicatorCalculationRequest is the body a caller sends to run an indicator script.
//
// AggregationInterval, ResultType and EndTime may each be left out: the calculation
// then reads five-minute candles, produces one number per indicator, and computes up
// to now — which is exactly what it did before any of the three could be declared.
type IndicatorCalculationRequest struct {
	Symbol              string    `json:"symbol"`
	AggregationInterval string    `json:"aggregationInterval"`
	CandleCount         int       `json:"candleCount"`
	EndTime             time.Time `json:"endTime"`
	Script              string    `json:"script"`
	ResultType          string    `json:"resultType"`
}

// ToRequestDto turns the request into the shape the domain accepts.
func (indicatorCalculationRequest IndicatorCalculationRequest) ToRequestDto() dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol:              indicatorCalculationRequest.Symbol,
		AggregationInterval: indicatorCalculationRequest.AggregationInterval,
		CandleCount:         indicatorCalculationRequest.CandleCount,
		EndTime:             indicatorCalculationRequest.EndTime,
		Script:              indicatorCalculationRequest.Script,
		ResultType:          indicatorCalculationRequest.ResultType,
	}
}
