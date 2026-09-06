package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ClosedTradeVo is one round trip that has already finished. It is a value rather
// than a domain model because there is nothing left to decide about it: the position
// it came from did the deciding, and this is what fell out.
type ClosedTradeVo struct {
	Direction  PositionDirectionVo
	EntryTime  time.Time
	EntryPrice decimal.Decimal
	ExitTime   time.Time
	ExitPrice  decimal.Decimal
	Stake      decimal.Decimal
	Profit     decimal.Decimal
}

// IsWin says whether this trade made money. Breaking exactly even is not a win — it
// returned the stake and nothing else, and counting it as one would flatter every
// strategy that trades a lot and earns nothing.
func (closedTradeVo ClosedTradeVo) IsWin() bool {
	return closedTradeVo.Profit.IsPositive()
}

// ToDto hands the trade on in the shape it leaves the domain in.
func (closedTradeVo ClosedTradeVo) ToDto() dto.ClosedTradeDto {
	return dto.ClosedTradeDto{
		Direction:  string(closedTradeVo.Direction),
		EntryTime:  closedTradeVo.EntryTime.UTC(),
		EntryPrice: closedTradeVo.EntryPrice,
		ExitTime:   closedTradeVo.ExitTime.UTC(),
		ExitPrice:  closedTradeVo.ExitPrice,
		Stake:      closedTradeVo.Stake,
		Profit:     closedTradeVo.Profit,
	}
}
