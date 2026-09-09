package domains

import (
	"slices"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// MarketCatalogDomain is every market the system recognises and the rules each one
// runs by. It is the one place a stored market name becomes behavior.
//
// It exists so that "which markets are there" is answered once, from settings, at
// the composition root — and so that recognising a third market is one more entry
// rather than one more branch in whoever happened to ask. Everything downstream
// holds this and asks it for a MarketDomain; nothing downstream ever compares a
// market to a constant.
//
// An unrecognised or absent name is not refused. Rows written before a market was
// ever recorded carry nothing, and the honest reading of nothing is the market this
// system had when they were written — refusing them instead would break reading data
// that was perfectly valid when it was stored.
type MarketCatalogDomain struct {
	rulesByMarket map[vo.MarketVo]vo.MarketRulesVo
}

// NewMarketCatalogDomain takes the rules for every recognised market. The fallback
// market is always recognised, whatever the caller passes, so MarketOf can never
// come back with a market that has no rules to read.
//
// It puts each market's stretches of trading into start order as it takes them, so
// that nothing downstream has to trust the order they were configured in. Counting
// buckets depends on that order — it merges a bucket two adjacent stretches share, and
// out of order it would merge the wrong pair — so the order is established once, here,
// rather than asserted in a comment wherever it is read.
func NewMarketCatalogDomain(rulesByMarket map[vo.MarketVo]vo.MarketRulesVo) MarketCatalogDomain {
	recognisedRules := make(map[vo.MarketVo]vo.MarketRulesVo, len(rulesByMarket)+1)
	for market, rules := range rulesByMarket {
		// Ordered into a copy of their own, so putting them in order here cannot reach
		// back into the settings they came from.
		orderedStretches := slices.Clone(rules.TradingSession.Stretches)
		slices.SortStableFunc(orderedStretches, func(
			oneStretch vo.TradingStretchVo, otherStretch vo.TradingStretchVo,
		) int {
			return int(oneStretch.StartOffset - otherStretch.StartOffset)
		})

		rules.TradingSession.Stretches = orderedStretches
		recognisedRules[market] = rules
	}

	if _, isRecognised := recognisedRules[fallbackMarket]; !isRecognised {
		recognisedRules[fallbackMarket] = vo.MarketRulesVo{}
	}

	return MarketCatalogDomain{rulesByMarket: recognisedRules}
}

// fallbackMarket is what an unrecognised or absent market name means. Its rules
// happen to be the zero value — never closes, no follow ceiling — which is exactly
// how this system behaved before it knew markets existed.
const fallbackMarket = vo.MarketCrypto

// MarketOf reads a stored market name and hands back the market it means, ready to
// be asked about opening hours, follow ceilings and its own calendar.
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

// IsRecognised reports whether this name is a market the system knows. It is what
// separates "you named a market that does not exist" — which is a request to refuse —
// from reading an old row that named nothing, which MarketOf quietly forgives.
func (marketCatalogDomain MarketCatalogDomain) IsRecognised(storedMarket string) bool {
	_, isRecognised := marketCatalogDomain.rulesByMarket[vo.MarketVo(storedMarket)]

	return isRecognised
}

// RecognisedMarkets is every market name this catalog answers to, for saying so when
// a caller names one that is not among them.
func (marketCatalogDomain MarketCatalogDomain) RecognisedMarkets() []string {
	// The order is fixed rather than taken from the map, so that a refusal reads the
	// same way twice running.
	recognisedMarkets := make([]string, 0, len(marketCatalogDomain.rulesByMarket))
	for _, market := range []vo.MarketVo{
		vo.MarketCrypto, vo.MarketTaiwanStock, vo.MarketTaiwanFutures,
	} {
		if _, isRecognised := marketCatalogDomain.rulesByMarket[market]; isRecognised {
			recognisedMarkets = append(recognisedMarkets, string(market))
		}
	}

	return recognisedMarkets
}
