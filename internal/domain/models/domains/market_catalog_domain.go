package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// MarketCatalogDomain is the single place a stored market name becomes rules, so adding a market is one entry rather than a new branch.
// Unrecognised or absent names read as the fallback market rather than being refused, so rows stored before markets existed stay readable.
type MarketCatalogDomain struct {
	rulesByMarket map[vo.MarketVo]vo.MarketRulesVo
}

// NewMarketCatalogDomain always recognises the fallback market so MarketOf never returns a market without rules.
func NewMarketCatalogDomain(rulesByMarket map[vo.MarketVo]vo.MarketRulesVo) MarketCatalogDomain {
	recognisedRules := make(map[vo.MarketVo]vo.MarketRulesVo, len(rulesByMarket)+1)
	for market, rules := range rulesByMarket {
		recognisedRules[market] = rules
	}

	if _, isRecognised := recognisedRules[fallbackMarket]; !isRecognised {
		recognisedRules[fallbackMarket] = vo.MarketRulesVo{}
	}

	return MarketCatalogDomain{rulesByMarket: recognisedRules}
}

// fallbackMarket's zero-value rules (never closes, no follow ceiling) match how the system behaved before markets existed.
const fallbackMarket = vo.MarketCrypto

func (marketCatalogDomain MarketCatalogDomain) MarketOf(storedMarket string) MarketDomain {
	market := vo.MarketVo(storedMarket)

	rules, isRecognised := marketCatalogDomain.rulesByMarket[market]
	if !isRecognised {
		return MarketDomain{
			value: fallbackMarket,
			rules: marketCatalogDomain.rulesByMarket[fallbackMarket],
		}
	}

	return MarketDomain{value: market, rules: rules}
}

// IsRecognised lets callers refuse an unknown market by name, while MarketOf silently forgives old rows.
func (marketCatalogDomain MarketCatalogDomain) IsRecognised(storedMarket string) bool {
	_, isRecognised := marketCatalogDomain.rulesByMarket[vo.MarketVo(storedMarket)]

	return isRecognised
}

func (marketCatalogDomain MarketCatalogDomain) RecognisedMarkets() []string {
	// Fixed order so a refusal reads the same every time.
	recognisedMarkets := make([]string, 0, len(marketCatalogDomain.rulesByMarket))
	for _, market := range []vo.MarketVo{vo.MarketCrypto, vo.MarketTaiwanStock} {
		if _, isRecognised := marketCatalogDomain.rulesByMarket[market]; isRecognised {
			recognisedMarkets = append(recognisedMarkets, string(market))
		}
	}

	return recognisedMarkets
}
