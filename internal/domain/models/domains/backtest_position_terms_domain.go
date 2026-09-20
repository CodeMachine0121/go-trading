package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestPositionTermsDomain is the terms one replay takes a position on: how much of
// the cash it stakes, how much of that is borrowed, what the venue charges, and where
// it gets out.
//
// The four arrived one per slice, and each arrival added a parameter to three
// constructors and a field to three models — because every one of them is only ever
// asked the same single question: *given this much cash, open a position here.* Asked
// together, they answer it once, and the account goes back to holding money rather
// than to knowing how a position is priced.
//
// Its zero value is the terms every replay had before any of them existed: stake
// everything, borrow nothing, pay nothing, simulate no exits. That falls out of the
// four zero values rather than being stated here, which is why a replay built without
// terms still walks.
//
// **This is where the next term goes.** A funding rate, a per-order minimum fee, a
// slippage model — each is another thing a position is taken on, and each lands as a
// field here plus a line in OpenFor. None of them touches the account, the walk, or
// any signature.
type BacktestPositionTermsDomain struct {
	sizing           PositionSizingDomain
	exitLevels       BacktestExitLevelsDomain
	leverage         BacktestLeverageDomain
	transactionCosts BacktestTransactionCostsDomain
}

// NewBacktestPositionTermsDomain gathers the four. It checks nothing, deliberately:
// each of them refused what it had to refuse when it was built, and a second opinion
// here would be a second place for the same rule to live.
func NewBacktestPositionTermsDomain(
	sizing PositionSizingDomain,
	exitLevels BacktestExitLevelsDomain,
	leverage BacktestLeverageDomain,
	transactionCosts BacktestTransactionCostsDomain,
) BacktestPositionTermsDomain {
	return BacktestPositionTermsDomain{
		sizing:           sizing,
		exitLevels:       exitLevels,
		leverage:         leverage,
		transactionCosts: transactionCosts,
	}
}

// NeverOpensAnything says whether these terms could never put anything down, whatever
// the account happens to hold at the time.
//
// It takes nothing because the answer depends only on the terms themselves — which is
// the whole point of them being one thing. Its caller is a replay being validated, and
// what it does with a yes is refuse at the door: terms that can never open a position
// produce a report card of a strategy that never traded, and every word on that screen
// points at the algorithm instead of at the two numbers that caused it.
func (backtestPositionTermsDomain BacktestPositionTermsDomain) NeverOpensAnything() bool {
	return backtestPositionTermsDomain.sizing.NeverStakesUnder(
		backtestPositionTermsDomain.transactionCosts, backtestPositionTermsDomain.leverage)
}

// OpenFor is the position these terms take on at that price with that much cash on
// hand — or nothing, when the cash will not stretch to one.
//
// Nothing is not an error. A stake the account cannot currently cover means this one
// opening does not happen; the replay carries on and may well afford the next. A price
// of zero means the same: there is nothing to buy in a market priced at nothing, and
// the alternative — dividing anyway — ends a whole replay over one bad candle.
//
// It is one call rather than "work out the stake, then open one that big" because the
// two halves are one decision. Given separately, a caller could take the stake and
// open something else, or take it and forget to check it was affordable — and the
// account, which is the caller, has no business knowing that a stake is a thing that
// gets worked out at all.
func (backtestPositionTermsDomain BacktestPositionTermsDomain) OpenFor(
	direction vo.PositionDirectionVo,
	entryTime time.Time,
	entryPrice decimal.Decimal,
	availableCash decimal.Decimal,
) (BacktestPositionDomain, bool) {
	stake, canStake := backtestPositionTermsDomain.sizing.StakeFor(
		availableCash, backtestPositionTermsDomain.transactionCosts,
		backtestPositionTermsDomain.leverage)
	if !canStake {
		return BacktestPositionDomain{}, false
	}

	return newBacktestPositionDomain(
		direction, entryTime, entryPrice, stake,
		backtestPositionTermsDomain.exitLevels,
		backtestPositionTermsDomain.leverage,
		backtestPositionTermsDomain.transactionCosts)
}
