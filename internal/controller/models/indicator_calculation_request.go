package models

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// IndicatorCalculationRequest is the body a caller sends to run an indicator script.
type IndicatorCalculationRequest struct {
	Symbol      string `json:"symbol"`
	CandleCount int    `json:"candleCount"`
	Script      string `json:"script"`
}

// ToRequestDto turns the request into the shape the domain accepts.
func (indicatorCalculationRequest IndicatorCalculationRequest) ToRequestDto() dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol:      indicatorCalculationRequest.Symbol,
		CandleCount: indicatorCalculationRequest.CandleCount,
		Script:      indicatorCalculationRequest.Script,
	}
}
