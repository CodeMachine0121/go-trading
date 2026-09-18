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
//
// Its two exit prices are settled once at entry for exactly that reason. They are also
// this position's own: a bet reopened after being stopped out measures its levels from
// the price it actually got, not from the one before it.
type BacktestPositionDomain struct {
	direction  vo.PositionDirectionVo
	entryTime  time.Time
	entryPrice decimal.Decimal
	stake      decimal.Decimal
	unitCount  decimal.Decimal
	exitPrices vo.ExitPricesVo
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
	exitLevels BacktestExitLevelsDomain,
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
		exitPrices: exitLevels.PricesFrom(direction, entryPrice),
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

// ExitOn is the finished round trip this candle forced, if it forced one: the candle
// reached one of the two prices this position settled on at entry.
//
// It is judged against the candle's high and low rather than its close, because a stop
// is reached during the bar. Reading the close instead would be pretending nothing
// happened inside it, and a bar with a low of 97 and a close of 101 would carry a
// position straight through a stop at 98.
//
// **The stop is asked first, and that decides candles that reached both.** A single
// candle's high and low cannot say which came first — that information is simply not
// in it — so both readings are defensible and only one of them never flatters the
// strategy. The cost of the other is a report card that speaks well of a strategy on
// exactly the bars where it is most doubtful.
//
// Whether this candle is the one the position was entered on is not asked here, and
// not asked anywhere: the walk examines the levels before applying the candle's
// signal, so a position born on a candle is first examined on the next. That rule is
// the ordering, not a comparison — a comparison of times is something a timezone, or
// two bars opening in the same second, can get wrong without ever failing.
//
// It hands back a whole finished trade rather than a price and a reason, because the
// three things its caller does next — record it, take the cash back, count it — all
// read fields of exactly this shape.
func (backtestPositionDomain BacktestPositionDomain) ExitOn(
	kCandle vo.KCandleVo, exitTime time.Time,
) (vo.ClosedTradeVo, bool) {
	// A stop sits against the position: below a long, above a short. So a long's is
	// reached by the candle's low and a short's by its high — where those prices sit
	// is settled by BacktestExitLevelsDomain.PricesFrom, and this is the other half
	// of the same thought.
	isShort := backtestPositionDomain.direction == vo.PositionDirectionShort
	candleHigh := decimal.NewFromFloat(kCandle.High)
	candleLow := decimal.NewFromFloat(kCandle.Low)

	if backtestPositionDomain.exitPrices.HasStopLoss &&
		reachedBy(candleHigh, candleLow, backtestPositionDomain.exitPrices.StopLossPrice, isShort) {
		return backtestPositionDomain.ClosedAt(
			exitTime, backtestPositionDomain.exitPrices.StopLossPrice,
			vo.TradeExitReasonStopLoss), true
	}

	if backtestPositionDomain.exitPrices.HasTakeProfit &&
		reachedBy(candleHigh, candleLow, backtestPositionDomain.exitPrices.TakeProfitPrice, !isShort) {
		return backtestPositionDomain.ClosedAt(
			exitTime, backtestPositionDomain.exitPrices.TakeProfitPrice,
			vo.TradeExitReasonTakeProfit), true
	}

	return vo.ClosedTradeVo{}, false
}

// reachedBy says whether a candle got as far as a level that sits above or below it.
//
// Touching exactly counts as reaching. A price that traded at the level is a price the
// order sitting there would have been filled at, and the alternative would let a stop
// survive the bar that hit it precisely.
//
// One expression rather than four, because all four cases — a long's stop, a long's
// target, a short's stop, a short's target — are the same two questions asked of a
// level that is either above or below. Written out four times, one of them would
// eventually be the one that got a comparison backwards.
func reachedBy(
	candleHigh decimal.Decimal, candleLow decimal.Decimal,
	level decimal.Decimal, isAbove bool,
) bool {
	if isAbove {
		return candleHigh.GreaterThanOrEqual(level)
	}

	return candleLow.LessThanOrEqual(level)
}

// ClosedAt turns this position into the finished round trip it leaves behind.
//
// Why it ended travels with it rather than being inferred by whoever built it: an
// exit price alone cannot say whether it was a stop or the signal that happened to
// land on the same figure.
func (backtestPositionDomain BacktestPositionDomain) ClosedAt(
	exitTime time.Time, exitPrice decimal.Decimal, exitReason vo.TradeExitReasonVo,
) vo.ClosedTradeVo {
	return vo.ClosedTradeVo{
		Direction:  backtestPositionDomain.direction,
		EntryTime:  backtestPositionDomain.entryTime,
		EntryPrice: backtestPositionDomain.entryPrice,
		ExitTime:   exitTime.UTC(),
		ExitPrice:  exitPrice,
		Stake:      backtestPositionDomain.stake,
		Profit:     backtestPositionDomain.ProfitAt(exitPrice),
		ExitReason: exitReason,
	}
}
