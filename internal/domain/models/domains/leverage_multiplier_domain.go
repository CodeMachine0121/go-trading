package domains

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// noLeverage is a position worth exactly what was put down for it. It is also what
// nothing at all is read as, so "no leverage" and "one times leverage" are the same
// arithmetic everywhere rather than two cases to keep in step.
var noLeverage = decimal.NewFromInt(1)

// LeverageMultiplierDomain is one leverage multiplier: whether it holds together, and
// whether it is a loan at all.
//
// Two places in this system take that figure — what a bot suggests putting on each
// round, and what one replay simulates. They are different things, and deliberately
// so: one is advice that is stored, the other is how a single run is modelled. But
// the figure is the same figure, and 0.5 typed into either has to come back with the
// same sentence. Two copies of a sentence is two sentences waiting to disagree, and
// this whole feature exists because of a pair that did.
//
// Its zero value borrows nothing, which is what a caller that was handed nothing has.
type LeverageMultiplierDomain struct {
	// multiplier is zero whenever nothing is borrowed, whether that came in as
	// nothing at all or as one times. Keeping one spelling for "no loan" is what lets
	// the zero value mean it too, with no flag beside it that could disagree.
	multiplier decimal.Decimal
}

// NewLeverageMultiplierDomain reads the figure as it was typed and settles every rule
// about it, so that nothing downstream asks again.
//
// Nothing at all and exactly one both mean no loan, and for the same reason: a
// position paid for in full has no lender, so there is nobody who could ever call it
// in. Below one is refused rather than quietly read as one — somebody who typed 0.5
// meant something by it, half a position most likely, and rounding that up would
// double what they asked for without telling them. A negative is the same mistake
// wearing a minus sign.
func NewLeverageMultiplierDomain(
	declaredMultiplier decimal.Decimal,
) (LeverageMultiplierDomain, error) {
	if declaredMultiplier.IsZero() {
		return LeverageMultiplierDomain{}, nil
	}

	if declaredMultiplier.LessThan(noLeverage) {
		return LeverageMultiplierDomain{}, fmt.Errorf("槓桿倍數不得小於 1 倍")
	}

	if declaredMultiplier.Equal(noLeverage) {
		return LeverageMultiplierDomain{}, nil
	}

	return LeverageMultiplierDomain{multiplier: declaredMultiplier}, nil
}

// IsBorrowed is whether somebody else's money is in a position of this size.
//
// The threshold is one, not zero — the line every caller of this is at risk of
// copying wrong. The absence of an exit distance or a cost rate is zero; the absence
// of leverage is one times, because a position paid for in full has no lender.
func (leverageMultiplierDomain LeverageMultiplierDomain) IsBorrowed() bool {
	return leverageMultiplierDomain.multiplier.IsPositive()
}

// Multiplier is what a stake is multiplied by to get what it actually exposes.
//
// Borrowing nothing answers one rather than zero, so that every formula reading it is
// written once and needs no branch for the ordinary case.
func (leverageMultiplierDomain LeverageMultiplierDomain) Multiplier() decimal.Decimal {
	if !leverageMultiplierDomain.multiplier.IsPositive() {
		return noLeverage
	}

	return leverageMultiplierDomain.multiplier
}
