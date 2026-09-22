package domains

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// maximumStakeScale is how many decimal places the largest affordable stake is cut to.
//
// The division below rarely terminates — a rate of 0.1425 makes one that never does —
// so it has to stop somewhere, and where it stops decides whether the account can end
// up a fraction below zero. Rounding to nearest can hand back a stake whose own cost
// no longer fits; cutting the tail off cannot. Sixteen places is far past any money
// anyone replays and still well inside what an exact decimal carries.
const maximumStakeScale = 16

// BacktestTransactionCostsDomain is what one replay pays for the act of trading, as
// two rates: what an opening costs and what a closing costs, each a percentage of the
// money that changed hands.
//
// It exists because every report card this system had produced until now described a
// world where trading is free. A Taiwan round trip costs 0.47% and a crypto one 0.2%,
// and the damage is proportional to how often the strategy trades: the same
// break-even script reads −1.9% over four round trips a year and **−61%** over two
// hundred. Those are not two views of one strategy. Sorting two strategies by a
// report card that leaves this out can put them in the wrong order.
//
// Its zero value is a replay that pays nothing, which is what every existing call is.
// That is not a flag beside the two rates — a flag can disagree with them, and nobody
// could say which to believe. It falls out of the arithmetic itself: the largest
// affordable stake divided by one is the cash itself, and nothing times zero is zero.
//
// It knows nothing about accounts, cash or positions. It answers three arithmetic
// questions and hands the numbers back; who loses the money, and when, is settled in
// exactly one place — see BacktestAccountDomain.
type BacktestTransactionCostsDomain struct {
	// Zero means no charge on that side. The exit rate is already settled by the
	// constructor: a caller who named only an entry rate is charged the same on the
	// way out.
	entryCostPercentage decimal.Decimal
	exitCostPercentage  decimal.Decimal
}

// NewBacktestTransactionCostsDomain reads the two rates this run was given.
//
// The exit rate falls back to the entry rate when it is left out, and that is the one
// place these two differ from the two exit distances beside them. A stop and a target
// are two different things — a floor and a ceiling — so neither could sensibly stand
// in for the other. These two are the two halves of one thing, the price of trading,
// and the markets say so: a crypto venue charges the same both ways, while Taiwan
// adds a tax on the way out. One rate typed once covers the first case; the second is
// the only reason there are two boxes at all.
//
// The cost of that choice is that "charged on the way in, free on the way out" cannot
// be expressed. No market works that way.
//
// Both rates are checked as they were declared, before the fallback, so a refused
// figure is never quietly adopted by the other side.
func NewBacktestTransactionCostsDomain(
	entryCostPercentage decimal.Decimal, exitCostPercentage decimal.Decimal,
) (BacktestTransactionCostsDomain, error) {
	if entryError := validatedCostPercentage(
		entryCostPercentage, "進場成本率"); entryError != nil {
		return BacktestTransactionCostsDomain{}, entryError
	}

	if exitError := validatedCostPercentage(
		exitCostPercentage, "出場成本率"); exitError != nil {
		return BacktestTransactionCostsDomain{}, exitError
	}

	if !exitCostPercentage.IsPositive() {
		exitCostPercentage = entryCostPercentage
	}

	return BacktestTransactionCostsDomain{
		entryCostPercentage: entryCostPercentage,
		exitCostPercentage:  exitCostPercentage,
	}, nil
}

// validatedCostPercentage is one side's rate, checked.
//
// Shared by both sides so that the same 101 typed into either comes back with the same
// sentence. It is a function rather than a method because it has to run before there
// is anything to be a method on.
//
// The wording deliberately differs from the one the exit distances use. A distance
// past a hundred percent puts the price below zero; a rate past a hundred percent
// charges more than the whole of what changed hands. Two different absurdities
// deserve two different sentences, and a reader who gets the wrong one goes looking
// in the wrong place.
func validatedCostPercentage(costPercentage decimal.Decimal, name string) error {
	if costPercentage.IsNegative() {
		return fmt.Errorf("%s不得為負——負的成本等於交易就送錢", name)
	}

	if costPercentage.GreaterThan(oneHundredPercent) {
		return fmt.Errorf("%s不得超過 100%%——成本不會超過成交金額本身", name)
	}

	return nil
}

// MaximumStakeFrom is the largest stake this account could put down and still afford
// the entry cost that comes with it.
//
// It is what makes the three sizing modes share one answer to "can this opening
// happen". Staking X costs X plus X times the rate, so "X plus its cost fits in the
// cash" and "X is no bigger than the cash divided by one plus the rate" are the same
// sentence — and the second one is a single number every mode can be measured against
// rather than a check each of them writes for itself.
//
// A replay paying nothing gets its cash back untouched, by an early return rather than
// by dividing by one. Dividing would be arithmetically the same and would still cut
// the result to a fixed number of places, and this method's whole promise to every
// existing replay is that it hands back *that* figure, not one equal to it.
func (backtestTransactionCostsDomain BacktestTransactionCostsDomain) MaximumStakeFrom(
	availableCash decimal.Decimal,
) decimal.Decimal {
	if !backtestTransactionCostsDomain.entryCostPercentage.IsPositive() {
		return availableCash
	}

	// Written as cash × 100 ÷ (100 + rate) rather than cash ÷ (1 + rate/100). They
	// are the same quotient, but this one stays in the units the caller typed and
	// divides exactly wherever the arithmetic allows — the other converts to a
	// fraction first and rounds twice.
	return availableCash.Mul(oneHundredPercent).
		DivRound(oneHundredPercent.Add(
			backtestTransactionCostsDomain.entryCostPercentage), maximumStakeScale+2).
		Truncate(maximumStakeScale)
}

// EntryCostFor is what opening a position of that size costs.
func (backtestTransactionCostsDomain BacktestTransactionCostsDomain) EntryCostFor(
	stake decimal.Decimal,
) decimal.Decimal {
	return stake.Mul(backtestTransactionCostsDomain.entryCostPercentage).
		Div(oneHundredPercent)
}

// ExitCostFor is what closing costs, given the money that actually changed hands.
//
// What changed hands is the position's units at the price it left at — **not** what
// the position was worth. The two are the same number for a long and different for a
// short, which makes reading the wrong one a mistake that only ever shows up on half
// the trades and never looks wrong on the page. Where that number comes from is
// settled by BacktestPositionDomain, which is the only thing holding the unit count.
//
// The charge is taken as a magnitude so that it is always money leaving. Prices
// arrive from outside this system, and one that came through negative would otherwise
// turn a cost into income — an error that makes a report card better and never fails.
func (backtestTransactionCostsDomain BacktestTransactionCostsDomain) ExitCostFor(
	tradedNotional decimal.Decimal,
) decimal.Decimal {
	return tradedNotional.Mul(backtestTransactionCostsDomain.exitCostPercentage).
		Div(oneHundredPercent).Abs()
}
