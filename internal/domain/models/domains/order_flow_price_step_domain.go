package domains

import (
	"errors"

	"github.com/shopspring/decimal"
)

// OrderFlowPriceStepDomain is how wide one footprint price level is; it works in decimals so a price on a level boundary (0.3 at width 0.1) lands on that level rather than the one below.
type OrderFlowPriceStepDomain struct {
	step decimal.Decimal
}

// NewOrderFlowPriceStepDomain keeps the shortest decimal that reads back as the float, so 0.1 is exactly one tenth.
func NewOrderFlowPriceStepDomain(value float64) (OrderFlowPriceStepDomain, error) {
	step := decimal.NewFromFloat(value)
	if !step.IsPositive() {
		return OrderFlowPriceStepDomain{}, errors.New("是訂單流價位寬度，必須大於零")
	}

	return OrderFlowPriceStepDomain{step: step}, nil
}

// LevelOf counts whole steps below the price: the price rounded down to a multiple of the width, as an index.
func (priceStepDomain OrderFlowPriceStepDomain) LevelOf(price decimal.Decimal) int64 {
	return price.Div(priceStepDomain.step).Floor().IntPart()
}

// PriceOf is the lower edge of a level, computed in decimals so 3 × 0.1 reads 0.3.
func (priceStepDomain OrderFlowPriceStepDomain) PriceOf(level int64) float64 {
	return decimal.NewFromInt(level).Mul(priceStepDomain.step).InexactFloat64()
}

func (priceStepDomain OrderFlowPriceStepDomain) Value() float64 {
	return priceStepDomain.step.InexactFloat64()
}

// Key names the width canonically, so 10 and 10.0 share remembered footprints.
func (priceStepDomain OrderFlowPriceStepDomain) Key() string {
	return priceStepDomain.step.String()
}
