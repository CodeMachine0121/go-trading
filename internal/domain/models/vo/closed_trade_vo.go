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
	// EntryCost and ExitCost are what this round trip paid for the act of trading,
	// at each end. Both are zero for a replay given no rates, which is what every
	// replay was before there were rates to give.
	//
	// They travel with the trade rather than only as a total, because a total answers
	// how much and never which ones — and the reader of this list wants to know
	// whether the charges are concentrated in the trades that barely moved.
	EntryCost decimal.Decimal
	ExitCost  decimal.Decimal
	// Profit is net of both charges: what the money actually did on this round trip.
	//
	// Gross would make a trade that gained less than it cost read as a gain, and the
	// win rate is worked out from exactly this field — so a strategy scalping a third
	// of a percent in a market charging nearly half of one would report itself as
	// winning most of the time while losing money every time. Anyone wanting the
	// price move on its own adds the two charges back.
	Profit decimal.Decimal
	// ExitReason is how this round trip came to an end. A replay given no exit
	// distances ends every one of them by signal, which is what every replay did
	// before there were distances to give.
	ExitReason TradeExitReasonVo
}

// IsWin says whether this trade made money. Breaking exactly even is not a win — it
// returned the stake and nothing else, and counting it as one would flatter every
// strategy script that trades a lot and earns nothing.
//
// Because the profit it reads is net, a round trip that moved in the right direction
// but not far enough to cover its own charges is not a win either. That is the whole
// point: it is not a win.
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
		EntryCost:  closedTradeVo.EntryCost,
		ExitCost:   closedTradeVo.ExitCost,
		Profit:     closedTradeVo.Profit,
		ExitReason: string(closedTradeVo.ExitReason),
	}
}
