package domains

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// BacktestSlippageDomain is how far every fill of a contract replay lands on the wrong
// side of the price it was aimed at: buying a little dearer, selling a little cheaper.
//
// Its zero value is no slippage at all, which is what leaving it blank means — the
// same reading the transaction costs have.
type BacktestSlippageDomain struct {
	percentage decimal.Decimal
}

func NewBacktestSlippageDomain(percentage decimal.Decimal) (BacktestSlippageDomain, error) {
	if percentage.IsNegative() {
		return BacktestSlippageDomain{}, fmt.Errorf("滑點不得為負——負的滑點等於每一筆都成交得比市價好")
	}

	if percentage.GreaterThan(oneHundredPercent) {
		return BacktestSlippageDomain{}, fmt.Errorf("滑點不得超過 100%%——那會讓賣出的成交價變成負數")
	}

	return BacktestSlippageDomain{percentage: percentage}, nil
}

// BuyingAt is what a buy aimed at that price actually fills at.
func (backtestSlippageDomain BacktestSlippageDomain) BuyingAt(price decimal.Decimal) decimal.Decimal {
	return price.Add(portionOf(price, backtestSlippageDomain.percentage))
}

// SellingAt is what a sell aimed at that price actually fills at.
func (backtestSlippageDomain BacktestSlippageDomain) SellingAt(price decimal.Decimal) decimal.Decimal {
	return price.Sub(portionOf(price, backtestSlippageDomain.percentage))
}
