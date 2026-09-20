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
// It carries **no coarseness, no script and no trading mode**. The trading strategy
// already says all three, so a body with room for them would be a body with two
// answers and no rule about which one wins.
type TradingStrategyBacktestRequest struct {
	Symbol    string    `json:"symbol"`
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
	// Leverage is how many times the stake each position is exposed to, and
	// MaintenanceMarginRate how little of that exposure may be left before the loan
	// is called in and the position is taken off at a loss of the whole stake.
	//
	// Asked for here rather than read off the trading strategy, unlike the trading
	// mode: how much somebody is willing to borrow is a fact about their account,
	// not about the rules being replayed. Nothing, zero or one means nothing is
	// borrowed; leaving only the rate out means the figure the venues actually use.
	Leverage              decimal.Decimal `json:"leverage"`
	MaintenanceMarginRate decimal.Decimal `json:"maintenanceMarginRate"`
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
		StartTime:            request.StartTime,
		EndTime:              request.EndTime,
		InitialCapital:       request.InitialCapital,
		PositionSizingMode:   request.PositionSizingMode,
		PositionSizingValue:  request.PositionSizingValue,
		StopLossPercentage:   request.StopLossPercentage,
		TakeProfitPercentage: request.TakeProfitPercentage,

		Leverage:              request.Leverage,
		MaintenanceMarginRate: request.MaintenanceMarginRate,

		EntryCostPercentage: request.EntryCostPercentage,
		ExitCostPercentage:  request.ExitCostPercentage,
	}
}
