package domains

import "github.com/shopspring/decimal"

// PlannedRiskDomain is what the trade stood to lose at its planned stop, the unit every R multiple is measured in.
type PlannedRiskDomain struct {
	risk decimal.NullDecimal
}

// NewPlannedRiskDomain has no risk without a planned stop.
func NewPlannedRiskDomain(
	averageEntryPrice decimal.Decimal, enteredQuantity decimal.Decimal, plannedStopLossPrice decimal.NullDecimal,
) PlannedRiskDomain {
	if !plannedStopLossPrice.Valid {
		return PlannedRiskDomain{}
	}

	distance := averageEntryPrice.Sub(plannedStopLossPrice.Decimal).Abs()

	return PlannedRiskDomain{risk: decimal.NullDecimal{Decimal: distance.Mul(enteredQuantity), Valid: true}}
}

func (plannedRiskDomain PlannedRiskDomain) Value() decimal.NullDecimal {
	return plannedRiskDomain.risk
}

// RMultipleOf is nil without a positive risk, since there is then no unit to measure in.
func (plannedRiskDomain PlannedRiskDomain) RMultipleOf(profit decimal.Decimal) *float64 {
	if !plannedRiskDomain.risk.Valid || !plannedRiskDomain.risk.Decimal.IsPositive() {
		return nil
	}

	rMultiple := profit.Div(plannedRiskDomain.risk.Decimal).InexactFloat64()

	return &rMultiple
}
