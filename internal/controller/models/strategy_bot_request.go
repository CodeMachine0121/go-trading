package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// StrategyBotRequest is the body a caller sends to build or rewrite a strategy bot.
//
// One shape serves both, because a rewrite replaces everything a bot is made of.
// Which bot is meant comes from the path, never from the body, and neither does who
// owns it — a body that could name its own owner is a body that could claim
// somebody else's.
//
// Nothing about a bot's life is here either: run state, when it is next due, what it
// last sent and why it halted are things that happen to a bot, not things a caller
// sets. A request with nowhere to put them is how "saving a bot cannot start one"
// is made true of the shape rather than of the code reading it.
type StrategyBotRequest struct {
	Name                   string                           `json:"name"`
	Symbol                 string                           `json:"symbol"`
	TriggerIntervalMinutes int                              `json:"triggerIntervalMinutes"`
	SignalSources          []StrategyBotSignalSourceRequest `json:"signalSources"`
	BuyCondition           StrategyBotConditionRequest      `json:"buyCondition"`
	SellCondition          StrategyBotConditionRequest      `json:"sellCondition"`
}

// StrategyBotSignalSourceRequest is one strategy script as it is to run inside this bot.
type StrategyBotSignalSourceRequest struct {
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

// StrategyBotConditionRequest is one condition, nested exactly as a person builds
// it: a comparison, or a group of conditions joined by an operator.
//
// It arrives nested rather than flat because that is the shape it is thought in.
// Asking a caller to send a list of nodes with parent references would mean the
// screen flattens a tree that this system immediately rebuilds — two chances to
// disagree about one condition.
type StrategyBotConditionRequest struct {
	Operator    string                        `json:"operator"`
	Conditions  []StrategyBotConditionRequest `json:"conditions"`
	SourceLabel string                        `json:"sourceLabel"`
	Signal      string                        `json:"signal"`
}

// ToWriteDto turns the request into the shape the domain accepts, taking which bot
// is meant from the argument. A zero identifier means a bot that does not exist yet.
//
// The owner is not taken here at all: it is settled by the application from whoever
// is signed in, and on a rewrite from what is already stored.
func (strategyBotRequest StrategyBotRequest) ToWriteDto(id uint) dto.StrategyBotWriteDto {
	return dto.StrategyBotWriteDto{
		ID:                     id,
		Name:                   strategyBotRequest.Name,
		Symbol:                 strategyBotRequest.Symbol,
		TriggerIntervalMinutes: strategyBotRequest.TriggerIntervalMinutes,
		SignalSources:          strategyBotRequest.signalSourceWriteDtos(),
		BuyCondition:           strategyBotRequest.BuyCondition.ToDto(),
		SellCondition:          strategyBotRequest.SellCondition.ToDto(),
	}
}

// signalSourceWriteDtos hands the sources on untouched, always as a list.
func (strategyBotRequest StrategyBotRequest) signalSourceWriteDtos() []dto.StrategyBotSignalSourceWriteDto {
	signalSourceWriteDtos := make(
		[]dto.StrategyBotSignalSourceWriteDto, 0, len(strategyBotRequest.SignalSources))

	for _, signalSource := range strategyBotRequest.SignalSources {
		parameterValues := make(
			[]dto.StrategyScriptParameterValueDto, 0, len(signalSource.ParameterValues))
		for _, parameterValue := range signalSource.ParameterValues {
			parameterValues = append(parameterValues, dto.StrategyScriptParameterValueDto{
				Name:  parameterValue.Name,
				Value: parameterValue.Value,
			})
		}

		signalSourceWriteDtos = append(signalSourceWriteDtos, dto.StrategyBotSignalSourceWriteDto{
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
func (strategyBotConditionRequest StrategyBotConditionRequest) ToDto() dto.StrategyBotConditionDto {
	conditionDtos := make(
		[]dto.StrategyBotConditionDto, 0, len(strategyBotConditionRequest.Conditions))
	for _, condition := range strategyBotConditionRequest.Conditions {
		conditionDtos = append(conditionDtos, condition.ToDto())
	}

	return dto.StrategyBotConditionDto{
		Operator:    strategyBotConditionRequest.Operator,
		Conditions:  conditionDtos,
		SourceLabel: strategyBotConditionRequest.SourceLabel,
		Signal:      strategyBotConditionRequest.Signal,
	}
}
