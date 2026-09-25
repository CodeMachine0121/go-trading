package domains

import "github.com/shopspring/decimal"

// OptionalFigureDomain keeps "not reported" distinct from zero for figures a market may not publish, with the add, judge and script-handoff rules in one place.
type OptionalFigureDomain struct {
	value decimal.NullDecimal
}

func NewOptionalFigureDomain(value decimal.NullDecimal) OptionalFigureDomain {
	return OptionalFigureDomain{value: value}
}

// Value keeps absent as absent.
func (optionalFigureDomain OptionalFigureDomain) Value() decimal.NullDecimal {
	return optionalFigureDomain.value
}

// Plus merges readings: absent plus absent stays absent, and absent plus a number is that number.
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

// IsNegative is false for an unreported figure.
func (optionalFigureDomain OptionalFigureDomain) IsNegative() bool {
	return optionalFigureDomain.value.Valid && optionalFigureDomain.value.Decimal.IsNegative()
}

// AsScriptFigure hands absent figures to scripts as zero, the one place the distinction is knowingly lost to avoid changing every script's input shape.
func (optionalFigureDomain OptionalFigureDomain) AsScriptFigure() float64 {
	if !optionalFigureDomain.value.Valid {
		return 0
	}

	return optionalFigureDomain.value.Decimal.InexactFloat64()
}
