package models

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// SymbolScreeningProfileRequest leaves optional figures out to take the defaults.
type SymbolScreeningProfileRequest struct {
	Name                           string                                `json:"name"`
	StrategyScriptID               uint                                  `json:"strategyScriptId"`
	ParameterValues                []StrategyScriptParameterValueRequest `json:"parameterValues"`
	AggregationInterval            string                                `json:"aggregationInterval"`
	TradingMode                    string                                `json:"tradingMode"`
	Leverage                       *decimal.Decimal                      `json:"leverage"`
	EntryCostPercentage            *decimal.Decimal                      `json:"entryCostPercentage"`
	SlippagePercentage             *decimal.Decimal                      `json:"slippagePercentage"`
	HistoryDays                    *int                                  `json:"historyDays"`
	SegmentCount                   *int                                  `json:"segmentCount"`
	MinimumTradeCount              *int                                  `json:"minimumTradeCount"`
	MinimumAverageDailyQuoteVolume *decimal.Decimal                      `json:"minimumAverageDailyQuoteVolume"`
	BaselineSymbols                []string                              `json:"baselineSymbols"`
}

func (request SymbolScreeningProfileRequest) ToWriteDto() dto.SymbolScreeningProfileWriteDto {
	parameterValueDtos := make([]dto.StrategyScriptParameterValueDto, 0, len(request.ParameterValues))
	for _, valueRequest := range request.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, valueRequest.ToValueDto())
	}

	return dto.SymbolScreeningProfileWriteDto{
		Name:                           request.Name,
		StrategyScriptID:               request.StrategyScriptID,
		ParameterValues:                parameterValueDtos,
		AggregationInterval:            request.AggregationInterval,
		TradingMode:                    request.TradingMode,
		Leverage:                       request.Leverage,
		EntryCostPercentage:            request.EntryCostPercentage,
		SlippagePercentage:             request.SlippagePercentage,
		HistoryDays:                    request.HistoryDays,
		SegmentCount:                   request.SegmentCount,
		MinimumTradeCount:              request.MinimumTradeCount,
		MinimumAverageDailyQuoteVolume: request.MinimumAverageDailyQuoteVolume,
		BaselineSymbols:                request.BaselineSymbols,
	}
}
