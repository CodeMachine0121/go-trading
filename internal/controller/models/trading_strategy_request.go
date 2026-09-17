package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// TradingStrategyRequest is the body a caller sends to build or rewrite a trading
// strategy.
//
// One shape serves both, because a rewrite replaces everything it is made of. Which
// one is meant comes from the path, never from the body, and neither does who owns
// it — a body that could name its own owner is a body that could claim somebody
// else's.
//
// Nothing about a machine is here: no market to watch, no interval, no run state.
// A request with nowhere to put them is how "these are rules, not a bot" is made
// true of the shape rather than of the code reading it.
type TradingStrategyRequest struct {
	Name string `json:"name"`
	// TradingMode is which set of rules these are written for: whether a sell means
	// get out into cash or face the other way. Leaving it out means always in the
	// market, which is how every set of rules saved before this field existed reads.
	TradingMode   string                               `json:"tradingMode"`
	SignalSources []TradingStrategySignalSourceRequest `json:"signalSources"`
	BuyCondition  TradingStrategyConditionRequest      `json:"buyCondition"`
	SellCondition TradingStrategyConditionRequest      `json:"sellCondition"`
}

// TradingStrategySignalSourceRequest is one strategy script as it is to run inside
// this trading strategy.
type TradingStrategySignalSourceRequest struct {
	Label               string                             `json:"label"`
	StrategyScriptID    uint                               `json:"strategyScriptId"`
	AggregationInterval string                             `json:"aggregationInterval"`
	ParameterValues     []StrategyBotParameterValueRequest `json:"parameterValues"`
}

// StrategyBotParameterValueRequest is what one knob is worth in one source.
type StrategyBotParameterValueRequest struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// TradingStrategyConditionRequest is one condition, nested exactly as a person builds
// it: a comparison, or a group of conditions joined by an operator.
//
// It arrives nested rather than flat because that is the shape it is thought in.
// Asking a caller to send a list of nodes with parent references would mean the
// screen flattens a tree that this system immediately rebuilds — two chances to
// disagree about one condition.
type TradingStrategyConditionRequest struct {
	Operator    string                            `json:"operator"`
	Conditions  []TradingStrategyConditionRequest `json:"conditions"`
	SourceLabel string                            `json:"sourceLabel"`
	Signal      string                            `json:"signal"`
}

// ToWriteDto turns the request into the shape the domain accepts, taking which
// trading strategy is meant from the argument. A zero identifier means one that does
// not exist yet.
//
// The owner is not taken here at all: it is settled by the application from whoever
// is signed in, and on a rewrite from what is already stored.
func (tradingStrategyRequest TradingStrategyRequest) ToWriteDto(id uint) dto.TradingStrategyWriteDto {
	return dto.TradingStrategyWriteDto{
		ID:            id,
		Name:          tradingStrategyRequest.Name,
		TradingMode:   tradingStrategyRequest.TradingMode,
		SignalSources: tradingStrategyRequest.signalSourceWriteDtos(),
		BuyCondition:  tradingStrategyRequest.BuyCondition.ToDto(),
		SellCondition: tradingStrategyRequest.SellCondition.ToDto(),
	}
}

// signalSourceWriteDtos hands the sources on untouched, always as a list.
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

// ToDto turns this condition and everything under it into the domain's shape. It
// recurses through itself, so a condition nested five deep needs no more code than
// one nested once.
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
