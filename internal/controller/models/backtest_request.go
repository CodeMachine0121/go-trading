package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// BacktestRequest is the body a caller sends to replay a strategy script over a stretch of
// market that has already happened.
//
// It carries no indicator value kind. A replay reads one number per candle — the
// signal — so there is nothing here for a caller to declare and nothing to get wrong.
// Like an indicator calculation, it either names a strategy script or carries an
// algorithm the caller wrote and has not saved — never both. It declares no kind of
// value either way: a replay reads signals, so there is nothing to choose.
type BacktestRequest struct {
	StrategyScriptID uint   `json:"strategyScriptId"`
	Symbol           string `json:"symbol"`
	// Script and Parameters describe an algorithm the caller wrote and has not
	// saved. They are read only when no strategy script is named.
	Script              string                           `json:"script"`
	Parameters          []StrategyScriptParameterRequest `json:"parameters"`
	AggregationInterval string                           `json:"aggregationInterval"`
	StartTime           time.Time                        `json:"startTime"`
	EndTime             time.Time                        `json:"endTime"`
	// ParameterValues are what the strategy script's knobs are worth this time, used for
	// this replay only and never written back.
	ParameterValues []StrategyScriptParameterValueRequest `json:"parameterValues"`
	InitialCapital  decimal.Decimal                       `json:"initialCapital"`
	// PositionSizingMode is how much each opening stakes, and PositionSizingValue the
	// figure that goes with it. Staking everything needs no figure, so a caller that
	// chose it may leave the figure out entirely.
	PositionSizingMode  string          `json:"positionSizingMode"`
	PositionSizingValue decimal.Decimal `json:"positionSizingValue"`
	// TradingMode is carried for the reason Leverage is: there is one set of rules
	// left, so declaring anything other than spot is refused rather than quietly read
	// as it.
	TradingMode string `json:"tradingMode"`
	// StopLossPercentage and TakeProfitPercentage are how far from its entry a
	// position may be wrong, and how far right is far enough. Leaving both out means
	// simulating no exits, which is what every replay did before these existed — so
	// a caller that says nothing gets exactly the report card it got before.
	StopLossPercentage   decimal.Decimal `json:"stopLossPercentage"`
	TakeProfitPercentage decimal.Decimal `json:"takeProfitPercentage"`
	// Leverage is carried only so that a caller still asking to borrow is told this
	// system does not. Nothing, zero and one all mean a position paid for in full,
	// which is what every replay here is; anything above one is refused outright.
	Leverage decimal.Decimal `json:"leverage"`
	// EntryCostPercentage and ExitCostPercentage are what this replay pays for the
	// act of trading, at each end, as a percentage of the money that changes hands.
	// Leaving both out means trading is free, which is what every replay assumed
	// before these existed — so a caller that says nothing gets exactly the report
	// card it got before. Leaving only the exit out means it costs the same as the
	// entry.
	EntryCostPercentage decimal.Decimal `json:"entryCostPercentage"`
	ExitCostPercentage  decimal.Decimal `json:"exitCostPercentage"`
}

// ToParameterWriteDtos hands on the knobs an unsaved algorithm declares.
func (backtestRequest BacktestRequest) ToParameterWriteDtos() []dto.StrategyScriptParameterWriteDto {
	parameterWriteDtos := make([]dto.StrategyScriptParameterWriteDto, 0, len(backtestRequest.Parameters))
	for _, parameterRequest := range backtestRequest.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, parameterRequest.ToWriteDto())
	}

	return parameterWriteDtos
}

// ToRequestDto turns the request into the shape the domain accepts.
func (backtestRequest BacktestRequest) ToRequestDto() dto.BacktestRequestDto {
	return dto.BacktestRequestDto{
		Symbol:               backtestRequest.Symbol,
		AggregationInterval:  backtestRequest.AggregationInterval,
		StartTime:            backtestRequest.StartTime,
		EndTime:              backtestRequest.EndTime,
		ParameterValues:      backtestRequest.parameterValueDtos(),
		InitialCapital:       backtestRequest.InitialCapital,
		PositionSizingMode:   backtestRequest.PositionSizingMode,
		PositionSizingValue:  backtestRequest.PositionSizingValue,
		TradingMode:          backtestRequest.TradingMode,
		StopLossPercentage:   backtestRequest.StopLossPercentage,
		TakeProfitPercentage: backtestRequest.TakeProfitPercentage,

		Leverage: backtestRequest.Leverage,

		EntryCostPercentage: backtestRequest.EntryCostPercentage,
		ExitCostPercentage:  backtestRequest.ExitCostPercentage,
	}
}

func (backtestRequest BacktestRequest) parameterValueDtos() []dto.StrategyScriptParameterValueDto {
	parameterValueDtos := make([]dto.StrategyScriptParameterValueDto, 0, len(backtestRequest.ParameterValues))
	for _, valueRequest := range backtestRequest.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, valueRequest.ToValueDto())
	}

	return parameterValueDtos
}
