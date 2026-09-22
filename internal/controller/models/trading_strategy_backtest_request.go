package models

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// TradingStrategyBacktestRequest is the body a caller sends to replay one of their
// trading strategies over a stretch of market that has already happened.
//
// Which trading strategy is meant comes from the path, never from the body.
//
// It carries **no coarseness and no script**. The trading strategy already says both,
// so a body with room for them would be a body with two answers and no rule about
// which one wins.
type TradingStrategyBacktestRequest struct {
	Symbol    string    `json:"symbol"`
	// TradingMode is carried for the reason Leverage is, and it has to be carried
	// **here too**: a field this body did not declare would be dropped in silence,
	// and a caller asking for another set of rules would get two hundred and a spot
	// report card — the one outcome this whole slice exists to prevent. Both replays
	// refuse it in the same words because both hand it to the same gate.
	TradingMode string    `json:"tradingMode"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	// InitialCapital is what the account starts with.
	InitialCapital decimal.Decimal `json:"initialCapital"`
	// PositionSizingMode is how much each opening stakes, and PositionSizingValue the
	// figure that goes with it. Staking everything needs no figure, so a caller that
	// chose it may leave the figure out entirely.
	PositionSizingMode  string          `json:"positionSizingMode"`
	PositionSizingValue decimal.Decimal `json:"positionSizingValue"`
	// StopLossPercentage and TakeProfitPercentage are the two exit distances this
	// run simulates. Unlike the coarseness and the trading mode, they are asked for
	// here: the trading strategy has no opinion about what its owner can sit
	// through, and the same set of rules is worth replaying against several answers.
	StopLossPercentage   decimal.Decimal `json:"stopLossPercentage"`
	TakeProfitPercentage decimal.Decimal `json:"takeProfitPercentage"`
	// Leverage is carried only so that a caller still asking to borrow is told this
	// system does not. Nothing, zero and one all mean a position paid for in full,
	// which is what every replay here is; anything above one is refused outright.
	Leverage decimal.Decimal `json:"leverage"`
	// EntryCostPercentage and ExitCostPercentage are what this replay pays for the
	// act of trading. Asked for here rather than read off the trading strategy for
	// the same reason the exit distances are: a set of rules has no opinion about
	// what its owner's broker charges.
	EntryCostPercentage decimal.Decimal `json:"entryCostPercentage"`
	ExitCostPercentage  decimal.Decimal `json:"exitCostPercentage"`
}

// ToRequestDto turns the request into the shape the domain accepts. The signal
// sources, the two conditions and the trading mode are not taken from here at all —
// they come from the trading strategy the path names.
func (request TradingStrategyBacktestRequest) ToRequestDto() dto.TradingStrategyBacktestRequestDto {
	return dto.TradingStrategyBacktestRequestDto{
		Symbol:               request.Symbol,
		TradingMode:          request.TradingMode,
		StartTime:            request.StartTime,
		EndTime:              request.EndTime,
		InitialCapital:       request.InitialCapital,
		PositionSizingMode:   request.PositionSizingMode,
		PositionSizingValue:  request.PositionSizingValue,
		StopLossPercentage:   request.StopLossPercentage,
		TakeProfitPercentage: request.TakeProfitPercentage,

		Leverage: request.Leverage,

		EntryCostPercentage: request.EntryCostPercentage,
		ExitCostPercentage:  request.ExitCostPercentage,
	}
}
