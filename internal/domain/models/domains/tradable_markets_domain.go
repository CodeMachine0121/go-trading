package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradableMarketsDomain is which kinds of trading a stored trading key may do, and therefore which kinds of bot may switch auto order on.
type TradableMarketsDomain struct {
	spotTradingEnabled     bool
	contractTradingEnabled bool
}

func NewTradableMarketsDomain(spotTradingEnabled bool, contractTradingEnabled bool) TradableMarketsDomain {
	return TradableMarketsDomain{
		spotTradingEnabled:     spotTradingEnabled,
		contractTradingEnabled: contractTradingEnabled,
	}
}

// NewTradableMarketsDomainOf reads the markets back from their stored spellings, ignoring unknown ones so they never grant anything.
func NewTradableMarketsDomainOf(tradableMarkets []string) TradableMarketsDomain {
	tradableMarketsDomain := TradableMarketsDomain{}
	for _, tradableMarket := range tradableMarkets {
		switch vo.TradableMarketVo(tradableMarket) {
		case vo.TradableMarketSpot:
			tradableMarketsDomain.spotTradingEnabled = true
		case vo.TradableMarketContract:
			tradableMarketsDomain.contractTradingEnabled = true
		}
	}

	return tradableMarketsDomain
}

func (tradableMarketsDomain TradableMarketsDomain) IsEmpty() bool {
	return !tradableMarketsDomain.spotTradingEnabled && !tradableMarketsDomain.contractTradingEnabled
}

func (tradableMarketsDomain TradableMarketsDomain) Covers(tradableMarket vo.TradableMarketVo) bool {
	switch tradableMarket {
	case vo.TradableMarketSpot:
		return tradableMarketsDomain.spotTradingEnabled
	case vo.TradableMarketContract:
		return tradableMarketsDomain.contractTradingEnabled
	default:
		return false
	}
}

// UncoveredBotMarketDataKinds names the stored bot kinds whose auto order must be switched off; blank is listed with spot because bots stored before kinds existed are spot bots.
func (tradableMarketsDomain TradableMarketsDomain) UncoveredBotMarketDataKinds() []string {
	uncoveredKinds := make([]string, 0, 3)
	if !tradableMarketsDomain.spotTradingEnabled {
		uncoveredKinds = append(uncoveredKinds, string(vo.MarketDataKindKCandle), "")
	}
	if !tradableMarketsDomain.contractTradingEnabled {
		uncoveredKinds = append(uncoveredKinds, string(vo.MarketDataKindContractKCandle))
	}

	return uncoveredKinds
}
