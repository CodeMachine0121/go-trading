package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestAccountDomain is what a replay is holding at any moment: the cash not
// staked, the one position that may be open, and the round trips already finished.
//
// Holding at most one position is not a check this makes but the shape it has — there
// is one position field, so there is nowhere for a second one to go.
type BacktestAccountDomain struct {
	positionSizing    PositionSizingDomain
	availableCash     decimal.Decimal
	openPosition      BacktestPositionDomain
	hasOpenPosition   bool
	positionOpenCount int
	closedTrades      []vo.ClosedTradeVo
}

func NewBacktestAccountDomain(
	initialCapital decimal.Decimal, positionSizing PositionSizingDomain,
) *BacktestAccountDomain {
	return &BacktestAccountDomain{
		positionSizing: positionSizing,
		availableCash:  initialCapital,
		closedTrades:   make([]vo.ClosedTradeVo, 0),
	}
}

// Apply carries out one candle's opinion at that candle's fill price.
//
// It is one method rather than "close this, then open that" because closing and
// reopening on the same candle is a single decision — reversing — and a caller given
// the two halves separately could reverse into a position while the old one was still
// counted, or forget the second half entirely.
//
// An opinion matching what is already held does nothing at all: no trade, no counted
// opening, no cash moved. Hearing "buy" twice is hearing it once.
func (backtestAccountDomain *BacktestAccountDomain) Apply(
	signal SignalDomain, candleTime time.Time, fillPrice decimal.Decimal,
) {
	wantedDirection, wantsPosition := signal.WantedDirection()
	if !wantsPosition {
		return
	}

	if backtestAccountDomain.hasOpenPosition {
		if backtestAccountDomain.openPosition.Direction() == wantedDirection {
			return
		}

		backtestAccountDomain.closedTrades = append(
			backtestAccountDomain.closedTrades,
			backtestAccountDomain.openPosition.ClosedAt(candleTime, fillPrice))
		backtestAccountDomain.availableCash = backtestAccountDomain.availableCash.Add(
			backtestAccountDomain.openPosition.ValueAt(fillPrice))
		backtestAccountDomain.hasOpenPosition = false
	}

	// An opening the account cannot afford simply does not happen: the replay carries
	// on flat, nothing is counted and nothing is reported. A strategy that outgrows
	// its own account is behaving, not failing.
	stake, canStake := backtestAccountDomain.positionSizing.StakeFor(
		backtestAccountDomain.availableCash)
	if !canStake {
		return
	}

	openedPosition, isOpened := NewBacktestPositionDomain(
		wantedDirection, candleTime, fillPrice, stake)
	if !isOpened {
		return
	}

	backtestAccountDomain.availableCash = backtestAccountDomain.availableCash.Sub(stake)
	backtestAccountDomain.openPosition = openedPosition
	backtestAccountDomain.hasOpenPosition = true
	backtestAccountDomain.positionOpenCount++
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

// ClosedTrades are the round trips that finished, earliest first. A position still
// open is not among them: it has no exit to report.
func (backtestAccountDomain *BacktestAccountDomain) ClosedTrades() []vo.ClosedTradeVo {
	return backtestAccountDomain.closedTrades
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
