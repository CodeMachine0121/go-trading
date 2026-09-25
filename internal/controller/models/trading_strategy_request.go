package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// TradingStrategyRequest serves create and full rewrite; the ID and owner never come from the body, and it holds rules only, no bot settings.
type TradingStrategyRequest struct {
	Name string `json:"name"`
	// MarketDataKind is kCandle (default) or contractKCandle, fixed at creation.
	MarketDataKind string `json:"marketDataKind"`
	// TradingMode is longShort (default when blank), longOnly or shortOnly for contract strategies; naming one on a K candle strategy is refused.
	TradingMode   string                               `json:"tradingMode"`
	SignalSources []TradingStrategySignalSourceRequest `json:"signalSources"`
	BuyCondition  TradingStrategyConditionRequest      `json:"buyCondition"`
	SellCondition TradingStrategyConditionRequest      `json:"sellCondition"`
}

type TradingStrategySignalSourceRequest struct {
	Label               string                             `json:"label"`
	StrategyScriptID    uint                               `json:"strategyScriptId"`
	AggregationInterval string                             `json:"aggregationInterval"`
	ParameterValues     []StrategyBotParameterValueRequest `json:"parameterValues"`
}

type StrategyBotParameterValueRequest struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// TradingStrategyConditionRequest arrives nested (a comparison or an operator-joined group), matching how conditions are built.
type TradingStrategyConditionRequest struct {
	Operator    string                            `json:"operator"`
	Conditions  []TradingStrategyConditionRequest `json:"conditions"`
	SourceLabel string                            `json:"sourceLabel"`
	Signal      string                            `json:"signal"`
}

// ToWriteDto takes the strategy ID from the argument (zero means new); the owner is set by the application.
func (tradingStrategyRequest TradingStrategyRequest) ToWriteDto(id uint) dto.TradingStrategyWriteDto {
	return dto.TradingStrategyWriteDto{
		ID:             id,
		Name:           tradingStrategyRequest.Name,
		TradingMode:    tradingStrategyRequest.TradingMode,
		MarketDataKind: tradingStrategyRequest.MarketDataKind,
		SignalSources:  tradingStrategyRequest.signalSourceWriteDtos(),
		BuyCondition:   tradingStrategyRequest.BuyCondition.ToDto(),
		SellCondition:  tradingStrategyRequest.SellCondition.ToDto(),
	}
}

func (tradingStrategyRequest TradingStrategyRequest) signalSourceWriteDtos() []dto.TradingStrategySignalSourceWriteDto {
	signalSourceWriteDtos := make(
		[]dto.TradingStrategySignalSourceWriteDto, 0, len(tradingStrategyRequest.SignalSources))

	for _, signalSource := range tradingStrategyRequest.SignalSources {
		parameterValues := make(
			[]dto.StrategyScriptParameterValueDto, 0, len(signalSource.ParameterValues))
		for _, parameterValue := range signalSource.ParameterValues {
			parameterValues = append(parameterValues, dto.StrategyScriptParameterValueDto{
				Name:  parameterValue.Name,
				Value: parameterValue.Value,
			})
		}

		signalSourceWriteDtos = append(signalSourceWriteDtos, dto.TradingStrategySignalSourceWriteDto{
			Label:               signalSource.Label,
			StrategyScriptID:    signalSource.StrategyScriptID,
			AggregationInterval: signalSource.AggregationInterval,
			ParameterValues:     parameterValues,
		})
	}

	return signalSourceWriteDtos
}

func (tradingStrategyConditionRequest TradingStrategyConditionRequest) ToDto() dto.TradingStrategyConditionDto {
	conditionDtos := make(
		[]dto.TradingStrategyConditionDto, 0, len(tradingStrategyConditionRequest.Conditions))
	for _, condition := range tradingStrategyConditionRequest.Conditions {
		conditionDtos = append(conditionDtos, condition.ToDto())
	}

	return dto.TradingStrategyConditionDto{
		Operator:    tradingStrategyConditionRequest.Operator,
		Conditions:  conditionDtos,
		SourceLabel: tradingStrategyConditionRequest.SourceLabel,
		Signal:      tradingStrategyConditionRequest.Signal,
	}
}
