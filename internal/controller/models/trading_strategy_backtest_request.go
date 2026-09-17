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
// It carries **no coarseness and no script**. The signal sources already say both, so
// a body with room for them would be a body with two answers and no rule about which
// one wins.
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
	// TradingMode is which set of rules this replay trades by. Leaving it out means
	// always being in the market.
	TradingMode string `json:"tradingMode"`
}

// ToRequestDto turns the request into the shape the domain accepts. The signal
// sources and the two conditions are not taken from here at all — they come from the
// trading strategy the path names.
func (request TradingStrategyBacktestRequest) ToRequestDto() dto.TradingStrategyBacktestRequestDto {
	return dto.TradingStrategyBacktestRequestDto{
		Symbol:              request.Symbol,
		StartTime:           request.StartTime,
		EndTime:             request.EndTime,
		InitialCapital:      request.InitialCapital,
		PositionSizingMode:  request.PositionSizingMode,
		PositionSizingValue: request.PositionSizingValue,
		TradingMode:         request.TradingMode,
	}
}
