package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// cryptoPrefillQuantityDecimals is how finely a prefilled crypto quantity is floored; the person corrects it to the venue's step.
const cryptoPrefillQuantityDecimals = 8

// SpotTradeMarketDomain is what a spot trade's market decides: the money it counts in and how its quantities are written.
type SpotTradeMarketDomain struct {
	market vo.MarketVo
}

// NewSpotTradeMarketDomain reads an absent or unknown market as crypto, as the symbol registry does.
func NewSpotTradeMarketDomain(market string) SpotTradeMarketDomain {
	if vo.MarketVo(market) == vo.MarketTaiwanStock {
		return SpotTradeMarketDomain{market: vo.MarketTaiwanStock}
	}

	return SpotTradeMarketDomain{market: vo.MarketCrypto}
}

func (marketDomain SpotTradeMarketDomain) Market() vo.MarketVo {
	return marketDomain.market
}

func (marketDomain SpotTradeMarketDomain) Currency() string {
	if marketDomain.market == vo.MarketTaiwanStock {
		return "TWD"
	}

	return "USDT"
}

// RequireQuantity refuses part of a share, since Taiwan stock is counted in whole shares.
func (marketDomain SpotTradeMarketDomain) RequireQuantity(quantity decimal.Decimal) error {
	if marketDomain.market == vo.MarketTaiwanStock && !quantity.Equal(quantity.Truncate(0)) {
		return fmt.Errorf("%w: 台股數量以股計，必須是整數", ErrSpotTradeValidation)
	}

	return nil
}

// PrefillQuantityOf floors what a stake buys at a price: whole shares for Taiwan stock, eight decimals for crypto.
func (marketDomain SpotTradeMarketDomain) PrefillQuantityOf(stake decimal.Decimal, price decimal.Decimal) decimal.Decimal {
	decimals := int32(cryptoPrefillQuantityDecimals)
	if marketDomain.market == vo.MarketTaiwanStock {
		decimals = 0
	}

	return stake.DivRound(price, decimals+4).Truncate(decimals)
}
