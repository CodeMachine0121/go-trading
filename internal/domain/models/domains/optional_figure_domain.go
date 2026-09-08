package domains

import "github.com/shopspring/decimal"

// OptionalFigureDomain is a trading figure a market may simply not report.
//
// It exists because "absent" and "zero" are different facts that look identical the
// moment either is written as a number. A minute in which nothing traded
// has a volume of zero; a market that does not publish turnover at all has no
// turnover figure — and an indicator built on the second one, told it was the first,
// produces a whole column of plausible wrong answers with nothing anywhere to
// complain about.
//
// Keeping the distinction costs one rule in three places — adding, judging, and
// handing to a script — so all three live here rather than being re-decided at each.
type OptionalFigureDomain struct {
	value decimal.NullDecimal
}

func NewOptionalFigureDomain(value decimal.NullDecimal) OptionalFigureDomain {
	return OptionalFigureDomain{value: value}
}

// Value is the figure as it is stored and handed out — absent stays absent.
func (optionalFigureDomain OptionalFigureDomain) Value() decimal.NullDecimal {
	return optionalFigureDomain.value
}

// Plus adds another reading of the same figure, which is what merging several
// candles into a coarser one does.
//
// Absent plus absent is absent: merging a market's non-existent turnover figures
// must not invent one. Absent plus a number is that number, because a market that
// reports the figure sometimes did trade that much — treating the missing readings
// as zero is the only reading that does not throw away what was reported.
func (optionalFigureDomain OptionalFigureDomain) Plus(added decimal.NullDecimal) OptionalFigureDomain {
	if !added.Valid {
		return optionalFigureDomain
	}

	if !optionalFigureDomain.value.Valid {
		return OptionalFigureDomain{value: added}
	}

	return OptionalFigureDomain{value: decimal.NullDecimal{
		Decimal: optionalFigureDomain.value.Decimal.Add(added.Decimal),
		Valid:   true,
	}}
}

// IsNegative reports a figure that breaks the rule that trading figures are never
// below zero. A figure that was never reported breaks no rule — there is nothing to
// judge.
func (optionalFigureDomain OptionalFigureDomain) IsNegative() bool {
	return optionalFigureDomain.value.Valid && optionalFigureDomain.value.Decimal.IsNegative()
}

// AsScriptFigure is this figure as an indicator script sees it. A script is handed
// plain numbers and has no way to say "not reported", so an absent figure arrives as
// zero.
//
// This is the one place the distinction is knowingly given up, and it is given up
// because the alternative — changing the shape every existing strategy reads — breaks
// them all to fix a market none of them are written against yet.
func (optionalFigureDomain OptionalFigureDomain) AsScriptFigure() float64 {
	if !optionalFigureDomain.value.Valid {
		return 0
	}

	return optionalFigureDomain.value.Decimal.InexactFloat64()
}
