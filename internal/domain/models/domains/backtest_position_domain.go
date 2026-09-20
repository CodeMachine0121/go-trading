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
	// stake is the money actually taken out of the account to hold this — the
	// margin. What the position is exposed to is that multiplied by the leverage,
	// and the two part company the moment anything is borrowed. Everything the
	// market touches is measured against the exposure; only this comes out of the
	// cash.
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
	leverage BacktestLeverageDomain,
	transactionCosts BacktestTransactionCostsDomain,
) (BacktestPositionDomain, bool) {
	if !entryPrice.IsPositive() || !stake.IsPositive() {
		return BacktestPositionDomain{}, false
	}

	// What the market moves is the exposure, not the stake, so both the units held
	// and the charge for taking them on are measured against it. A replay that
	// borrows nothing gets its stake back from ExposureFrom unchanged, which is what
	// keeps every figure on an unleveraged report card exactly where it was.
	exposure := leverage.ExposureFrom(stake)

	return BacktestPositionDomain{
		direction:        direction,
		entryTime:        entryTime.UTC(),
		entryPrice:       entryPrice,
		stake:            stake,
		unitCount:        exposure.Div(entryPrice),
		exitPrices:       exitLevels.PricesFrom(direction, entryPrice, leverage),
		transactionCosts: transactionCosts,
		entryCost:        transactionCosts.EntryCostFor(exposure),
	}, true
}

// EntryCost is what was already paid to open this bet. A replay given no rates paid
// nothing, and answers zero.
func (backtestPositionDomain BacktestPositionDomain) EntryCost() decimal.Decimal {
	return backtestPositionDomain.entryCost
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
// **The adverse level is asked first, and that decides candles that reached both.** A
// single candle's high and low cannot say which came first — that information is
// simply not in it — so both readings are defensible and only one of them never
// flatters the strategy. The cost of the other is a report card that speaks well of a
// strategy on exactly the bars where it is most doubtful.
//
// There is one adverse level rather than a stop and a liquidation asked in turn,
// because the two sit on the same side and only the nearer can ever be reached. Which
// that is was settled when the position opened — see
// BacktestExitLevelsDomain.PricesFrom — so this asks one question and reads the answer
// it was given, rather than re-deciding on every bar.
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
	// An adverse level sits against the position: below a long, above a short. So a
	// long's is reached by the candle's low and a short's by its high — where that
	// price sits is settled by BacktestExitLevelsDomain.PricesFrom, and this is the
	// other half of the same thought.
	isShort := backtestPositionDomain.direction == vo.PositionDirectionShort
	candleHigh := decimal.NewFromFloat(kCandle.High)
	candleLow := decimal.NewFromFloat(kCandle.Low)

	if backtestPositionDomain.exitPrices.HasAdverse &&
		reachedBy(candleHigh, candleLow, backtestPositionDomain.exitPrices.AdversePrice, isShort) {
		return backtestPositionDomain.ClosedAt(
			exitTime, backtestPositionDomain.exitPrices.AdversePrice,
			backtestPositionDomain.exitPrices.AdverseReason), true
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
// One expression rather than four, because all four cases — a long's adverse level, a
// long's target, a short's adverse level, a short's target — are the same two questions
// asked of a level that is either above or below. Written out four times, one of them
// would eventually be the one that got a comparison backwards.
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
//
// What it cost to get out is worked out here, against the money that actually changed
// hands: the units this bet holds, at the price it left at. That is **not** what the
// position was worth — for a short, leaving a hundred units at 90 hands over 9000
// while the bet itself is worth 11000. Charging the second figure would be wrong on
// every short and on no long, which is the kind of wrong nobody finds by reading the
// numbers.
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
	profit := backtestPositionDomain.ProfitAt(exitPrice).
		Sub(backtestPositionDomain.entryCost).Sub(exitCost)

	// A loan called in takes the whole margin and charges nothing on the way out.
	//
	// Not because the arithmetic says so — at the liquidation price the position is
	// still worth the maintenance margin — but because that is what is left for the
	// venue to take, and it takes it. Reporting anything coming back would say the
	// margin survived, and the one thing everybody knows about being liquidated is
	// that it did not. The exit charge goes for the same reason it is never charged
	// twice: there is no money left to pay it with.
	//
	// What the account actually receives is CashReturnedFor, below. This sets what
	// the finished trade says it cost, and the two agree because both read this one
	// decision.
	if exitReason == vo.TradeExitReasonLiquidation {
		exitCost = decimal.Zero
		profit = backtestPositionDomain.stake.Add(backtestPositionDomain.entryCost).Neg()
	}

	return vo.ClosedTradeVo{
		Direction:  backtestPositionDomain.direction,
		EntryTime:  backtestPositionDomain.entryTime,
		EntryPrice: backtestPositionDomain.entryPrice,
		ExitTime:   exitTime.UTC(),
		ExitPrice:  exitPrice,
		Stake:      backtestPositionDomain.stake,
		EntryCost:  backtestPositionDomain.entryCost,
		ExitCost:   exitCost,
		Profit:     profit,
		ExitReason: exitReason,
	}
}

// CashReturnedFor is what the account gets back for letting this position go.
//
// It is the one way out. The account used to work this figure out itself, as the
// position's value less the charge for leaving — which was right while there was only
// one way to leave. A loan called in is a second way, and it answers differently in
// two places at once: nothing comes back, and nothing more is charged. Two exceptions
// living in the account would have split "what one position is worth on the way out"
// across two models, and the day a third way to leave arrives, split it again.
//
// Never negative. A position cannot cost more to hold than was put behind it: the
// adverse level is reached before the money runs out, and when a candle gaps straight
// past it the exit still fills there. The floor is the guard for the arithmetic having
// been asked at a price further out than any exit — which nothing does today, and
// which nobody should have to prove again before adding the next reason to close.
func (backtestPositionDomain BacktestPositionDomain) CashReturnedFor(
	closedTrade vo.ClosedTradeVo,
) decimal.Decimal {
	if closedTrade.ExitReason == vo.TradeExitReasonLiquidation {
		return decimal.Zero
	}

	cashReturned := backtestPositionDomain.ValueAt(closedTrade.ExitPrice).Sub(closedTrade.ExitCost)
	if cashReturned.IsNegative() {
		return decimal.Zero
	}

	return cashReturned
}
