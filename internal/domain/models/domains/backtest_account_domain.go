package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestAccountDomain is what a replay is holding at any moment: the cash not
// staked, the one position that may be open, and the round trips already finished.
//
// Holding at most one position is not a check this makes but the shape it has — there
// is one position field, so there is nowhere for a second one to go.
type BacktestAccountDomain struct {
	positionTerms     BacktestPositionTermsDomain
	availableCash     decimal.Decimal
	openPosition      BacktestPositionDomain
	hasOpenPosition   bool
	positionOpenCount int
	closedTrades      []vo.ClosedTradeVo
}

func NewBacktestAccountDomain(
	initialCapital decimal.Decimal,
	positionTerms BacktestPositionTermsDomain,
) *BacktestAccountDomain {
	return &BacktestAccountDomain{
		positionTerms: positionTerms,
		availableCash: initialCapital,
		closedTrades:  make([]vo.ClosedTradeVo, 0),
	}
}

// ApplyExitLevels closes the open position if this candle reached one of the two
// prices it settled on at entry. A replay given no distances does nothing here at all.
//
// It is called before the candle's own signal, and that ordering carries two rules on
// its own. A position opened on a candle is first examined on the next one, because
// this ran before it existed — so "the entry candle cannot stop itself out" needs no
// check anywhere. And a position stopped out here still hears that candle's signal
// afterwards, which is right: the stop was reached during the bar and the close came
// after it. Swallowing the signal as well would let one stop eat an entry that had
// nothing to do with it.
func (backtestAccountDomain *BacktestAccountDomain) ApplyExitLevels(
	kCandle vo.KCandleVo, candleTime time.Time,
) {
	if !backtestAccountDomain.hasOpenPosition {
		return
	}

	closedTrade, isExited := backtestAccountDomain.openPosition.ExitOn(kCandle, candleTime)
	if !isExited {
		return
	}

	backtestAccountDomain.settleOpenPosition(closedTrade)
}

// Apply carries out one candle's opinion at that candle's fill price.
//
// It is one method rather than "close this, then open that" because what to let go of
// and what to take on is a single decision, and a caller given the two halves
// separately could forget the second one.
//
// What the opinion asks for is the signal's own answer, not this method's — see
// SignalDomain.TargetPosition. Reading it as a target rather than as a signal is what
// keeps the walk below free of any branch about which word arrived.
//
// An opinion asking for what is already held does nothing at all: no trade, no
// counted opening, no cash moved. Hearing "buy" twice is hearing it once, and so is
// hearing "sell" with nothing to sell.
func (backtestAccountDomain *BacktestAccountDomain) Apply(
	signal SignalDomain, candleTime time.Time, fillPrice decimal.Decimal,
) {
	targetPosition := signal.TargetPosition()
	// Having no opinion is not the same as asking for cash, and this is the line that
	// keeps them apart: an unchanged target leaves an open position alone, where a
	// flat one would go on to close it.
	if targetPosition == vo.TargetPositionUnchanged {
		return
	}

	wantsPosition := targetPosition.WantsPosition()

	if backtestAccountDomain.hasOpenPosition {
		if wantsPosition {
			return
		}

		backtestAccountDomain.settleOpenPosition(
			backtestAccountDomain.openPosition.ClosedAt(
				candleTime, fillPrice, vo.TradeExitReasonSignal))
	}

	// Cash was what it asked for, and cash is what it now holds. This is where a sell
	// stops, and the only reason the opening below is not reached by every target.
	if !wantsPosition {
		return
	}

	// An opening the account cannot afford simply does not happen: the replay carries
	// on flat, nothing is counted and nothing is reported. A strategy script that
	// outgrows its own account is behaving, not failing.
	//
	// How big it would have been, whether that was affordable, what the venue charges
	// and where it gets out are all one answer from the terms. The account holds money
	// and a position; it has no business knowing that a stake is a thing that gets
	// worked out.
	openedPosition, isOpened := backtestAccountDomain.positionTerms.OpenFor(
		candleTime, fillPrice, backtestAccountDomain.availableCash)
	if !isOpened {
		return
	}

	// The stake and what it cost to put it down leave together. They are one
	// withdrawal in two parts, and the terms have already guaranteed both fit.
	backtestAccountDomain.availableCash = backtestAccountDomain.availableCash.
		Sub(openedPosition.Stake()).Sub(openedPosition.EntryCost())
	backtestAccountDomain.openPosition = openedPosition
	backtestAccountDomain.hasOpenPosition = true
	backtestAccountDomain.positionOpenCount++
}

// settleOpenPosition is the whole of letting go of a position: the trade joins the
// list, the cash it is worth comes back, and the account is flat again.
//
// It is a method rather than four lines written twice because both ways out — the
// signal asking for something else, and a candle reaching a level — have to do all
// four. One of two copies missing the cash line is money appearing or vanishing, and
// nothing downstream would report it as anything but a very good or very bad strategy.
//
// How much comes back is the position's answer rather than this one's, so that "what
// one position is worth on the way out" lives in one model — the day a way out answers
// differently, it lands there instead of adding a case to this method.
func (backtestAccountDomain *BacktestAccountDomain) settleOpenPosition(
	closedTrade vo.ClosedTradeVo,
) {
	backtestAccountDomain.closedTrades = append(
		backtestAccountDomain.closedTrades, closedTrade)
	backtestAccountDomain.availableCash = backtestAccountDomain.availableCash.
		Add(backtestAccountDomain.openPosition.CashReturnedFor(closedTrade))
	backtestAccountDomain.hasOpenPosition = false
}

// EquityAt is what everything on hand is worth at that price: the cash, plus any open
// position valued as though it were closed there.
func (backtestAccountDomain *BacktestAccountDomain) EquityAt(
	price decimal.Decimal,
) decimal.Decimal {
	if !backtestAccountDomain.hasOpenPosition {
		return backtestAccountDomain.availableCash
	}

	return backtestAccountDomain.availableCash.Add(
		backtestAccountDomain.openPosition.ValueAt(price))
}

// ClosedTradeDtos are the round trips that finished, earliest first, in the shape they
// leave the domain in. A position still open is not among them: it has no exit to
// report.
func (backtestAccountDomain *BacktestAccountDomain) ClosedTradeDtos() []dto.ClosedTradeDto {
	closedTradeDtos := make([]dto.ClosedTradeDto, 0, len(backtestAccountDomain.closedTrades))
	for _, closedTrade := range backtestAccountDomain.closedTrades {
		closedTradeDtos = append(closedTradeDtos, closedTrade.ToDto())
	}

	return closedTradeDtos
}

// ExitCountFor is how many finished round trips ended that way.
//
// It is counted off the trade list rather than tallied as it goes, for the reason the
// win rate is: a counter is a second place the same fact lives, and the day it
// disagrees with the list nobody can say which one to believe.
func (backtestAccountDomain *BacktestAccountDomain) ExitCountFor(
	exitReason vo.TradeExitReasonVo,
) int {
	exitCount := 0
	for _, closedTrade := range backtestAccountDomain.closedTrades {
		if closedTrade.ExitReason == exitReason {
			exitCount++
		}
	}

	return exitCount
}

// TotalTransactionCost is everything paid for the act of trading so far: both charges
// on every finished round trip, plus the entry charge on a position still open.
//
// A position still open counts because that money is already gone — it left when the
// position was opened. What it will cost to close is not here, because it has not
// been paid and this figure only ever reports money that has moved.
//
// It is added up off the trade list rather than tallied as it goes, for the reason the
// win rate is: a running total is a second place the same fact lives, and the day it
// disagrees with the list nobody can say which one to believe.
func (backtestAccountDomain *BacktestAccountDomain) TotalTransactionCost() decimal.Decimal {
	totalTransactionCost := decimal.Zero
	for _, closedTrade := range backtestAccountDomain.closedTrades {
		totalTransactionCost = totalTransactionCost.
			Add(closedTrade.EntryCost).Add(closedTrade.ExitCost)
	}

	if backtestAccountDomain.hasOpenPosition {
		totalTransactionCost = totalTransactionCost.Add(
			backtestAccountDomain.openPosition.EntryCost())
	}

	return totalTransactionCost
}

// PositionOpenCount is how many openings actually happened. One that was skipped for
// want of cash is not one of them, and the position still open at the end is.
func (backtestAccountDomain *BacktestAccountDomain) PositionOpenCount() int {
	return backtestAccountDomain.positionOpenCount
}

// WinRate is the share of finished round trips that made money, and whether it means
// anything at all.
//
// Nothing finished and every trade lost are two different statements. Answering the
// first with a rate of zero would make them look like one, so the second answer here
// is what tells them apart — and it is answered together with the rate, because a
// caller that had to ask twice could use the number without ever asking.
func (backtestAccountDomain *BacktestAccountDomain) WinRate() (float64, bool) {
	if len(backtestAccountDomain.closedTrades) == 0 {
		return 0, false
	}

	winCount := 0
	for _, closedTrade := range backtestAccountDomain.closedTrades {
		if closedTrade.IsWin() {
			winCount++
		}
	}

	return float64(winCount) / float64(len(backtestAccountDomain.closedTrades)), true
}
