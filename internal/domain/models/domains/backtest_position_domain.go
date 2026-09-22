package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestPositionDomain is the one bet a replay is holding: when and at what price it
// was entered, what was staked on it, and how many units that bought.
//
// **It faces one way.** A replay only ever trades spot — cash for goods — so there is
// no direction to carry and no branch anywhere below that asks about one. Every
// finished trade reports itself as long because every one of them is.
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
	entryTime  time.Time
	entryPrice decimal.Decimal
	// stake is the money taken out of the account to hold this, and also everything
	// this bet has at risk. Spot borrows nothing, so the two are one figure.
	stake      decimal.Decimal
	unitCount  decimal.Decimal
	exitPrices vo.ExitPricesVo
	// transactionCosts travels with the position for the same reason the exit prices
	// do: what this bet costs to get out of is settled by the rates the run was given,
	// and reading them from somewhere else later would let a position be charged at a
	// rate it was never opened under.
	transactionCosts BacktestTransactionCostsDomain
	// entryCost is money already gone, worked out once at entry and kept.
	//
	// Kept rather than derived again for the reason the unit count is: a second
	// multiplication is a second chance to disagree with the first, and this figure is
	// the only record that the account has already paid it. The running total of what
	// a replay spent asks a position still open what it cost to open, and this is the
	// answer.
	entryCost decimal.Decimal
}

// newBacktestPositionDomain opens a position at a candle's close.
//
// It is unexported so that BacktestPositionTermsDomain.OpenFor is the only way a
// position comes into being. A second door taking a stake already worked out is an
// invitation for a caller to work one out — and the point of the terms is that
// nobody outside them knows a stake is something that gets decided.
//
// A non-positive entry price is refused rather than divided by. There is nothing to
// buy in a market priced at zero, and the alternative — dividing anyway — ends the
// whole replay with a panic over one bad candle.
func newBacktestPositionDomain(
	entryTime time.Time,
	entryPrice decimal.Decimal,
	stake decimal.Decimal,
	exitLevels BacktestExitLevelsDomain,
	transactionCosts BacktestTransactionCostsDomain,
) (BacktestPositionDomain, bool) {
	if !entryPrice.IsPositive() || !stake.IsPositive() {
		return BacktestPositionDomain{}, false
	}

	return BacktestPositionDomain{
		entryTime:        entryTime.UTC(),
		entryPrice:       entryPrice,
		stake:            stake,
		unitCount:        stake.Div(entryPrice),
		exitPrices:       exitLevels.PricesFrom(entryPrice),
		transactionCosts: transactionCosts,
		entryCost:        transactionCosts.EntryCostFor(stake),
	}, true
}

// Stake is the money taken out of the account to hold this.
//
// The account asks because it is the account's money; how much that turned out to be
// is the terms' decision, and this is where the answer comes back.
func (backtestPositionDomain BacktestPositionDomain) Stake() decimal.Decimal {
	return backtestPositionDomain.stake
}

// EntryCost is what was already paid to open this bet. A replay given no rates paid
// nothing, and answers zero.
func (backtestPositionDomain BacktestPositionDomain) EntryCost() decimal.Decimal {
	return backtestPositionDomain.entryCost
}

// ProfitAt is what this position would have made or lost had it been closed at that
// price: what the price gained, on every unit held.
func (backtestPositionDomain BacktestPositionDomain) ProfitAt(
	price decimal.Decimal,
) decimal.Decimal {
	return backtestPositionDomain.unitCount.Mul(
		price.Sub(backtestPositionDomain.entryPrice))
}

// ValueAt is what this position is worth in cash at that price: the stake back, plus
// whatever it has made or lost. It is the same figure whether the position is being
// closed or merely valued at the end of a candle, which is why an open position and a
// closed one contribute to the equity curve through one expression rather than two.
//
// It is deliberately gross of the cost of getting out. That cost has not been paid
// while the position is open, and taking it off every candle would draw an equity
// curve describing something that has not happened. The account subtracts it at the
// one moment it becomes real — see BacktestAccountDomain.settleOpenPosition. The
// price of that honesty is that a replay ending with a position still open reports a
// final equity one exit charge too kind, which the report card says out loud.
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
	// The stop sits under the position and the target above it, so the candle's low
	// reaches one and its high reaches the other — the other half of the thought in
	// BacktestExitLevelsDomain.PricesFrom.
	candleHigh := decimal.NewFromFloat(kCandle.High)
	candleLow := decimal.NewFromFloat(kCandle.Low)

	if backtestPositionDomain.exitPrices.HasStopLoss &&
		candleLow.LessThanOrEqual(backtestPositionDomain.exitPrices.StopLossPrice) {
		return backtestPositionDomain.ClosedAt(
			exitTime, backtestPositionDomain.exitPrices.StopLossPrice,
			vo.TradeExitReasonStopLoss), true
	}

	if backtestPositionDomain.exitPrices.HasTakeProfit &&
		candleHigh.GreaterThanOrEqual(backtestPositionDomain.exitPrices.TakeProfitPrice) {
		return backtestPositionDomain.ClosedAt(
			exitTime, backtestPositionDomain.exitPrices.TakeProfitPrice,
			vo.TradeExitReasonTakeProfit), true
	}

	return vo.ClosedTradeVo{}, false
}

// ClosedAt turns this position into the finished round trip it leaves behind.
//
// Why it ended travels with it rather than being inferred by whoever built it: an
// exit price alone cannot say whether it was a stop or the signal that happened to
// land on the same figure.
//
// What it cost to get out is worked out against the money that actually changed
// hands: the units this bet holds, at the price it left at.
//
// The profit it leaves behind is net of both charges, because that is what the money
// actually did. Everything downstream reads it that way without being told: a round
// trip that gained less than it cost stops counting as a win, and the win rate stops
// flattering a strategy that scalps a third of a percent in a market that charges
// nearly half of one. Both charges travel alongside it, so gross is always one
// addition away.
func (backtestPositionDomain BacktestPositionDomain) ClosedAt(
	exitTime time.Time, exitPrice decimal.Decimal, exitReason vo.TradeExitReasonVo,
) vo.ClosedTradeVo {
	exitCost := backtestPositionDomain.transactionCosts.ExitCostFor(
		backtestPositionDomain.unitCount.Mul(exitPrice))

	return vo.ClosedTradeVo{
		Direction:  vo.PositionDirectionLong,
		EntryTime:  backtestPositionDomain.entryTime,
		EntryPrice: backtestPositionDomain.entryPrice,
		ExitTime:   exitTime.UTC(),
		ExitPrice:  exitPrice,
		Stake:      backtestPositionDomain.stake,
		EntryCost:  backtestPositionDomain.entryCost,
		ExitCost:   exitCost,
		Profit: backtestPositionDomain.ProfitAt(exitPrice).
			Sub(backtestPositionDomain.entryCost).Sub(exitCost),
		ExitReason: exitReason,
	}
}

// CashReturnedFor is what the account gets back for letting this position go: what the
// bet is worth at the price it left at, less the charge for leaving.
//
// It is asked of the position rather than worked out by the account, so that "what one
// position is worth on the way out" lives in one model. The day there is a second way
// to leave that answers differently, it lands here rather than becoming a second case
// in the account.
//
// **It does not floor.** A spot position cannot be worth less than nothing — the goods
// are worth what they are worth — so the subtraction has nowhere to go below zero that
// the numbers did not already go.
func (backtestPositionDomain BacktestPositionDomain) CashReturnedFor(
	closedTrade vo.ClosedTradeVo,
) decimal.Decimal {
	return backtestPositionDomain.ValueAt(closedTrade.ExitPrice).Sub(closedTrade.ExitCost)
}
