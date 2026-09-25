package domains

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// BacktestSlippageDomain moves every contract fill against the trader by a percentage; the zero value means no slippage.
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

func (backtestSlippageDomain BacktestSlippageDomain) BuyingAt(price decimal.Decimal) decimal.Decimal {
	return price.Add(portionOf(price, backtestSlippageDomain.percentage))
}

func (backtestSlippageDomain BacktestSlippageDomain) SellingAt(price decimal.Decimal) decimal.Decimal {
	return price.Sub(portionOf(price, backtestSlippageDomain.percentage))
}
