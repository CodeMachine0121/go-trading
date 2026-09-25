package domains

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// contractPriceLineDomain is one venue-computed OHLC line on a contract K candle, checked for completeness and high >= low; sign rules stay with the candle.
type contractPriceLineDomain struct {
	open  decimal.Decimal
	high  decimal.Decimal
	low   decimal.Decimal
	close decimal.Decimal
}

func newContractPriceLineDomain(
	lineName string, open, high, low, close decimal.NullDecimal,
) (contractPriceLineDomain, error) {
	for _, figure := range []decimal.NullDecimal{open, high, low, close} {
		if !figure.Valid {
			return contractPriceLineDomain{}, fmt.Errorf(
				"%w: %s必填", ErrKCandleContractValidation, lineName)
		}
	}

	if high.Decimal.LessThan(low.Decimal) {
		return contractPriceLineDomain{}, fmt.Errorf(
			"%w: %s最高不得低於最低", ErrKCandleContractValidation, lineName)
	}

	return contractPriceLineDomain{
		open: open.Decimal, high: high.Decimal, low: low.Decimal, close: close.Decimal,
	}, nil
}

// hasNegativeFigure checks all four figures because the high/low ordering says nothing about open and close.
func (contractPriceLineDomain contractPriceLineDomain) hasNegativeFigure() bool {
	for _, figure := range []decimal.Decimal{
		contractPriceLineDomain.open, contractPriceLineDomain.high,
		contractPriceLineDomain.low, contractPriceLineDomain.close,
	} {
		if figure.IsNegative() {
			return true
		}
	}

	return false
}
