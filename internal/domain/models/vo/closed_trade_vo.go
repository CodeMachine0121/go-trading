package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ClosedTradeVo is one finished round trip.
type ClosedTradeVo struct {
	Direction  PositionDirectionVo
	EntryTime  time.Time
	EntryPrice decimal.Decimal
	ExitTime   time.Time
	ExitPrice  decimal.Decimal
	Stake      decimal.Decimal
	// EntryCost and ExitCost are the trading charges at each end, zero when no rates were given.
	EntryCost decimal.Decimal
	ExitCost  decimal.Decimal
	// Profit is net of both charges so the win rate cannot count a trade that lost to fees as a win.
	Profit decimal.Decimal
	// ExitReason is always signal when no exit distances were given.
	ExitReason TradeExitReasonVo
}

// IsWin reports a strictly positive net profit; breaking even or losing to fees is not a win.
func (closedTradeVo ClosedTradeVo) IsWin() bool {
	return closedTradeVo.Profit.IsPositive()
}

func (closedTradeVo ClosedTradeVo) ToDto() dto.ClosedTradeDto {
	return dto.ClosedTradeDto{
		Direction:  string(closedTradeVo.Direction),
		EntryTime:  closedTradeVo.EntryTime.UTC(),
		EntryPrice: closedTradeVo.EntryPrice,
		ExitTime:   closedTradeVo.ExitTime.UTC(),
		ExitPrice:  closedTradeVo.ExitPrice,
		Stake:      closedTradeVo.Stake,
		EntryCost:  closedTradeVo.EntryCost,
		ExitCost:   closedTradeVo.ExitCost,
		Profit:     closedTradeVo.Profit,
		ExitReason: string(closedTradeVo.ExitReason),
	}
}

// ToOutcomeVo adds both charges back to the net profit for the trade statistics.
func (closedTradeVo ClosedTradeVo) ToOutcomeVo() TradeOutcomeVo {
	transactionCost := closedTradeVo.EntryCost.Add(closedTradeVo.ExitCost)

	return TradeOutcomeVo{
		NetProfit:       closedTradeVo.Profit,
		GrossProfit:     closedTradeVo.Profit.Add(transactionCost),
		TransactionCost: transactionCost,
		EntryTime:       closedTradeVo.EntryTime,
		ExitTime:        closedTradeVo.ExitTime,
	}
}
