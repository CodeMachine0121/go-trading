package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestPositionDomain is the one bet a replay is holding: which way it faces, when
// and at what price it was entered, what was staked on it, and how many units that
// bought.
//
// The unit count is worked out once at entry and kept, rather than derived again at
// every valuation. Deriving it again would divide by the entry price on every candle,
// and the day one of those divisions rounds differently from another, an unchanged
// position would appear to drift.
type BacktestPositionDomain struct {
	direction  vo.PositionDirectionVo
	entryTime  time.Time
	entryPrice decimal.Decimal
	stake      decimal.Decimal
	unitCount  decimal.Decimal
}

// NewBacktestPositionDomain opens a position at a candle's close.
//
// A non-positive entry price is refused rather than divided by. There is nothing to
// buy in a market priced at zero, and the alternative — dividing anyway — ends the
// whole replay with a panic over one bad candle.
func NewBacktestPositionDomain(
	direction vo.PositionDirectionVo,
	entryTime time.Time,
	entryPrice decimal.Decimal,
	stake decimal.Decimal,
) (BacktestPositionDomain, bool) {
	if !entryPrice.IsPositive() || !stake.IsPositive() {
		return BacktestPositionDomain{}, false
	}

	return BacktestPositionDomain{
		direction:  direction,
		entryTime:  entryTime.UTC(),
		entryPrice: entryPrice,
		stake:      stake,
		unitCount:  stake.Div(entryPrice),
	}, true
}

func (backtestPositionDomain BacktestPositionDomain) Direction() vo.PositionDirectionVo {
	return backtestPositionDomain.direction
}

// ProfitAt is what this position would have made or lost had it been closed at that
// price. A long earns what the price gained and a short earns what it lost, which is
// the whole of the difference between the two directions.
func (backtestPositionDomain BacktestPositionDomain) ProfitAt(
	price decimal.Decimal,
) decimal.Decimal {
	priceMovement := price.Sub(backtestPositionDomain.entryPrice)
	if backtestPositionDomain.direction == vo.PositionDirectionShort {
		priceMovement = priceMovement.Neg()
	}

	return backtestPositionDomain.unitCount.Mul(priceMovement)
}

// ValueAt is what this position is worth in cash at that price: the stake back, plus
// whatever it has made or lost. It is the same figure whether the position is being
// closed or merely valued at the end of a candle, which is why an open position and a
// closed one contribute to the equity curve through one expression rather than two.
func (backtestPositionDomain BacktestPositionDomain) ValueAt(
	price decimal.Decimal,
) decimal.Decimal {
	return backtestPositionDomain.stake.Add(backtestPositionDomain.ProfitAt(price))
}

// ClosedAt turns this position into the finished round trip it leaves behind.
func (backtestPositionDomain BacktestPositionDomain) ClosedAt(
	exitTime time.Time, exitPrice decimal.Decimal,
) vo.ClosedTradeVo {
	return vo.ClosedTradeVo{
		Direction:  backtestPositionDomain.direction,
		EntryTime:  backtestPositionDomain.entryTime,
		EntryPrice: backtestPositionDomain.entryPrice,
		ExitTime:   exitTime.UTC(),
		ExitPrice:  exitPrice,
		Stake:      backtestPositionDomain.stake,
		Profit:     backtestPositionDomain.ProfitAt(exitPrice),
	}
}
