package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractBacktestRequest is a spot replay request plus leverage, contract trading mode and slippage; it names a strategy script or carries one, never both.
type ContractBacktestRequest struct {
	StrategyScriptID     uint                                  `json:"strategyScriptId"`
	Script               string                                `json:"script"`
	Parameters           []StrategyScriptParameterRequest      `json:"parameters"`
	Symbol               string                                `json:"symbol"`
	AggregationInterval  string                                `json:"aggregationInterval"`
	StartTime            time.Time                             `json:"startTime"`
	EndTime              time.Time                             `json:"endTime"`
	ParameterValues      []StrategyScriptParameterValueRequest `json:"parameterValues"`
	InitialCapital       decimal.Decimal                       `json:"initialCapital"`
	PositionSizingMode   string                                `json:"positionSizingMode"`
	PositionSizingValue  decimal.Decimal                       `json:"positionSizingValue"`
	StopLossPercentage   decimal.Decimal                       `json:"stopLossPercentage"`
	TakeProfitPercentage decimal.Decimal                       `json:"takeProfitPercentage"`
	EntryCostPercentage  decimal.Decimal                       `json:"entryCostPercentage"`
	ExitCostPercentage   decimal.Decimal                       `json:"exitCostPercentage"`
	// FillTiming is close (default, the signalling bar's close) or nextOpen (the next bar's open).
	FillTiming string `json:"fillTiming"`
	// ValidationStartTime, when given, splits the replay into in-sample and validation parts, each starting from the initial capital.
	ValidationStartTime time.Time `json:"validationStartTime"`
	// Leverage blank or zero is one.
	Leverage decimal.Decimal `json:"leverage"`
	// TradingMode is longShort (the default), longOnly or shortOnly.
	TradingMode string `json:"tradingMode"`
	// SlippagePercentage blank is none.
	SlippagePercentage decimal.Decimal `json:"slippagePercentage"`
	// MaintenanceMarginRate is carried only to be refused — see the request DTO.
	MaintenanceMarginRate decimal.Decimal `json:"maintenanceMarginRate"`
}

func (request ContractBacktestRequest) ToParameterWriteDtos() []dto.StrategyScriptParameterWriteDto {
	parameterWriteDtos := make([]dto.StrategyScriptParameterWriteDto, 0, len(request.Parameters))
	for _, parameterRequest := range request.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, parameterRequest.ToWriteDto())
	}

	return parameterWriteDtos
}

func (request ContractBacktestRequest) ToRequestDto() dto.ContractBacktestRequestDto {
	parameterValueDtos := make([]dto.StrategyScriptParameterValueDto, 0, len(request.ParameterValues))
	for _, valueRequest := range request.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, valueRequest.ToValueDto())
	}

	return dto.ContractBacktestRequestDto{
		Symbol:                request.Symbol,
		AggregationInterval:   request.AggregationInterval,
		StartTime:             request.StartTime,
		EndTime:               request.EndTime,
		ParameterValues:       parameterValueDtos,
		InitialCapital:        request.InitialCapital,
		PositionSizingMode:    request.PositionSizingMode,
		PositionSizingValue:   request.PositionSizingValue,
		StopLossPercentage:    request.StopLossPercentage,
		TakeProfitPercentage:  request.TakeProfitPercentage,
		EntryCostPercentage:   request.EntryCostPercentage,
		ExitCostPercentage:    request.ExitCostPercentage,
		FillTiming:            request.FillTiming,
		ValidationStartTime:   request.ValidationStartTime,
		Leverage:              request.Leverage,
		TradingMode:           request.TradingMode,
		SlippagePercentage:    request.SlippagePercentage,
		MaintenanceMarginRate: request.MaintenanceMarginRate,
	}
}
