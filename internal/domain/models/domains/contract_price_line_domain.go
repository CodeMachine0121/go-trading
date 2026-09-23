package domains

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// contractPriceLineDomain is one of the lines a contract K candle carries beside its
// traded prices — an open, a high, a low and a close the venue computes on its own —
// with the two rules every such line answers to: all four were given, and the high
// is not below the low.
//
// It is unexported because it is a part of a contract K candle and nothing else: it
// exists so that "every line is complete and ordered" is written once for the lines
// that share it, with each line's own name in the sentence a caller is refused with.
// What differs between lines — whether a figure may be negative — stays with the
// candle, which is the one that knows what each line means.
type contractPriceLineDomain struct {
	open  decimal.Decimal
	high  decimal.Decimal
	low   decimal.Decimal
	close decimal.Decimal
}

// newContractPriceLineDomain checks the four figures of the line called lineName.
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

// hasNegativeFigure says whether any of the four sits below zero. The low is not
// enough to look at: the constructor keeps it at or below the high, but it says
// nothing about the open and the close.
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
