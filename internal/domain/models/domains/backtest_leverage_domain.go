package domains

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// noBacktestLeverage is a position worth exactly what was put down for it, written as
// the multiplier a caller would type. It is also what a replay with no leverage at all
// multiplies by, so "no leverage" and "one times leverage" are the same arithmetic
// everywhere below rather than two cases to keep in step.
var noBacktestLeverage = decimal.NewFromInt(1)

// defaultMaintenanceMarginRate is what a replay is charged with holding when it opened
// leverage and said nothing about where the line is.
//
// It is a default where the two models beside this one — the exit distances and the
// cost rates — deliberately have none, and the difference is the point. Leaving those
// blank means "do not simulate this at all", which is a thing a replay can honestly
// do. Leaving this blank cannot mean that: once money is borrowed, somebody is
// watching the collateral, and a replay that pretends otherwise is the exact lie this
// whole model exists to stop telling. So blank means "no opinion" and gets the figure
// the venues actually use.
var defaultMaintenanceMarginRate = decimal.NewFromFloat(0.5)

// BacktestLeverageDomain is how much one replay borrows against what it puts down, and
// how far a position may fall before the loan is called in.
//
// Its zero value is a replay that borrows nothing: exposure is the stake itself and
// there is no forced exit to simulate. That is what every existing replay is, and it
// is deliberately not a flag beside the two figures — a flag can disagree with them,
// and nobody could say which to believe. It falls out of the arithmetic instead:
// multiplying by one changes nothing, and a replay with no loan has no lender.
//
// Which is also why one times leverage is read as no leverage rather than as a loan of
// nothing. The reason a position gets closed against its owner's wishes is that
// somebody else's money is in it. Nobody else's money is in a position paid for in
// full, so there is nobody to call it in — at any price, however far it falls.
type BacktestLeverageDomain struct {
	// multiplier being zero is what makes this the zero value, so there is no separate
	// flag that could contradict the figures. A replay that borrows nothing has no
	// margin to maintain, and that fact cannot disagree with itself.
	multiplier            decimal.Decimal
	maintenanceMarginRate decimal.Decimal
}

// NewBacktestLeverageDomain reads the two figures this run was given and settles every
// rule about them, so that nothing downstream checks again.
//
// The trading mode is an argument because "spot cannot borrow" is a rule about
// leverage, not a rule about spot. Keeping it here is what stops this model from
// having one of its own rules living somewhere else.
func NewBacktestLeverageDomain(
	declaredMultiplier decimal.Decimal,
	declaredMaintenanceMarginRate decimal.Decimal,
	tradingMode TradingModeDomain,
) (BacktestLeverageDomain, error) {
	// Checked as it was declared, before anything decides whether it will be used —
	// the same order the cost rates are read in, and for the same reason: a figure
	// this would have refused must never be quietly adopted later. A negative rate is
	// a typo whether or not this particular replay had a use for it.
	if declaredMaintenanceMarginRate.IsNegative() {
		return BacktestLeverageDomain{}, fmt.Errorf(
			"維持保證金率不得為負——負的維持保證金等於倉位賠光了還撐得住")
	}

	// Nothing typed at all. Not a loan of nothing, not a loan of one: no loan.
	if declaredMultiplier.IsZero() {
		return BacktestLeverageDomain{}, nil
	}

	// Below one is refused rather than read as none. Somebody who typed 0.5 meant
	// something by it — half a position, probably — and quietly reading that as a
	// whole one would double what they asked for without telling them. The same
	// sentence a bot's position plan uses, because the same figure has to be answered
	// the same way wherever it is typed.
	if declaredMultiplier.LessThan(noBacktestLeverage) {
		return BacktestLeverageDomain{}, fmt.Errorf("槓桿倍數不得小於 1 倍")
	}

	// Exactly one is a position paid for in full. See the type's own comment: there is
	// no lender, so there is nothing to simulate, and saying so here is what keeps
	// every caller below from asking.
	if declaredMultiplier.Equal(noBacktestLeverage) {
		return BacktestLeverageDomain{}, nil
	}

	if !tradingMode.CanUseLeverage() {
		return BacktestLeverageDomain{}, fmt.Errorf(
			"%s交易模式開不了槓桿——現貨是拿現金換東西，沒有人借錢給你", tradingMode.InWords())
	}

	maintenanceMarginRate := declaredMaintenanceMarginRate
	if maintenanceMarginRate.IsZero() {
		maintenanceMarginRate = defaultMaintenanceMarginRate
	}

	// The loan itself decides how far the position may fall: put down a fifth of what
	// is exposed and a fifth is all there is to lose. What the venue keeps back on top
	// of that comes out of the same allowance.
	//
	// Non-positive means the position is already past the line on the candle it opened
	// on. A venue would not let that order through, and letting it through here
	// produces a report card where every single position is wiped out on entry — a
	// page of zeros that looks like a broken system rather than like a refused figure.
	maximumMaintenanceMarginRate := oneHundredPercent.Div(declaredMultiplier)
	if maintenanceMarginRate.GreaterThanOrEqual(maximumMaintenanceMarginRate) {
		return BacktestLeverageDomain{}, fmt.Errorf(
			"維持保證金率必須小於 %s%%——%s 倍槓桿下，押下去的錢只夠讓價格逆著走這麼多，"+
				"再多這一注在開倉那一棒就已經撐不住",
			maximumMaintenanceMarginRate.String(), declaredMultiplier.String())
	}

	return BacktestLeverageDomain{
		multiplier:            declaredMultiplier,
		maintenanceMarginRate: maintenanceMarginRate,
	}, nil
}

// Multiplier is what a stake is multiplied by to get what it actually exposes.
//
// A replay that borrows nothing answers one rather than zero, so that every formula
// reading it — what a position is worth, what the venue charges, how much of the cash
// can be staked — is written once and needs no branch for the ordinary case.
func (backtestLeverageDomain BacktestLeverageDomain) Multiplier() decimal.Decimal {
	if !backtestLeverageDomain.multiplier.IsPositive() {
		return noBacktestLeverage
	}

	return backtestLeverageDomain.multiplier
}

// ExposureFrom is what putting that much down actually puts at risk.
//
// Everything the market touches is measured against this rather than against the
// stake: what the position gains and loses, and what the venue charges at each end. A
// replay that borrows nothing gets its stake back unchanged, which is why the models
// downstream never ask whether there is leverage.
func (backtestLeverageDomain BacktestLeverageDomain) ExposureFrom(
	stake decimal.Decimal,
) decimal.Decimal {
	return stake.Mul(backtestLeverageDomain.Multiplier())
}

// IsBorrowed is whether somebody else's money is in these positions.
//
// It is asked by the one rule that may only apply to a borrowed position: that a
// loss cannot exceed the margin. A position paid for in full has no such rule and
// never had one — a short bought back at three times what it sold for really does
// cost more than it staked, and every report card made before there was anything to
// borrow says so. Capping that would rewrite them.
func (backtestLeverageDomain BacktestLeverageDomain) IsBorrowed() bool {
	return backtestLeverageDomain.multiplier.IsPositive()
}

// AdverseDistance is how far the price may move against a position before the loan is
// called in, as a percentage of the entry price — and whether that can happen at all.
//
// It is a distance rather than a price for the same reason the two exit distances are:
// it is the same number whichever way the position faces and whatever it was entered
// at, and it is the form that can be compared with a stop without knowing either.
//
// A replay that borrows nothing answers no, and that is the only answer that keeps
// every existing report card the way it was.
func (backtestLeverageDomain BacktestLeverageDomain) AdverseDistance() (decimal.Decimal, bool) {
	if !backtestLeverageDomain.multiplier.IsPositive() {
		return decimal.Zero, false
	}

	return oneHundredPercent.Div(backtestLeverageDomain.multiplier).
		Sub(backtestLeverageDomain.maintenanceMarginRate), true
}
