package domains

import (
	"maps"
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// LiveFollowRosterDomain is the single answer to which symbols of rostered markets are followed live, used by both the follower and the console so they never disagree.
// On-demand markets hold nothing here, capped markets give places to the earliest watched symbols, and uncapped ones roster every watched symbol.
type LiveFollowRosterDomain struct {
	// Kept so acting on a place needs no second, possibly different, lookup.
	marketsBySymbol map[string]vo.MarketVo
	// Read once from the catalogue for the same reason.
	symbolsPerChannelByMarket map[vo.MarketVo]int
}

// NewLiveFollowRosterDomain rosters watched symbols of open rostered markets in watchlist order up to each market's cap; shut markets hold none so the cap is left for the morning.
func NewLiveFollowRosterDomain(
	watchedSymbols []entities.TradingSymbol,
	marketCatalogDomain MarketCatalogDomain,
	currentTime time.Time,
) LiveFollowRosterDomain {
	placesLeft := make(map[vo.MarketVo]int)
	marketsBySymbol := make(map[string]vo.MarketVo)
	symbolsPerChannelByMarket := make(map[vo.MarketVo]int)

	for _, watchedSymbol := range watchedSymbols {
		marketDomain := marketCatalogDomain.MarketOf(watchedSymbol.Market)

		if !marketDomain.FollowsFixedRoster() {
			continue
		}

		if !marketDomain.IsOpen(currentTime) {
			continue
		}

		if marketDomain.HasFollowCeiling() {
			remaining, hasCounted := placesLeft[marketDomain.Value()]
			if !hasCounted {
				remaining = marketDomain.SimultaneousFollowCeiling()
			}

			if remaining <= 0 {
				placesLeft[marketDomain.Value()] = 0

				continue
			}

			placesLeft[marketDomain.Value()] = remaining - 1
		}

		marketsBySymbol[watchedSymbol.Symbol] = marketDomain.Value()
		symbolsPerChannelByMarket[marketDomain.Value()] = marketDomain.SymbolsPerLiveChannel()
	}

	return LiveFollowRosterDomain{
		marketsBySymbol:           marketsBySymbol,
		symbolsPerChannelByMarket: symbolsPerChannelByMarket,
	}
}

// Holds reports whether this symbol was given one of its market's places.
func (liveFollowRosterDomain LiveFollowRosterDomain) Holds(symbol string) bool {
	_, holdsAPlace := liveFollowRosterDomain.marketsBySymbol[symbol]

	return holdsAPlace
}

func (liveFollowRosterDomain LiveFollowRosterDomain) MarketOf(symbol string) vo.MarketVo {
	return liveFollowRosterDomain.marketsBySymbol[symbol]
}

// Symbols is in no particular order.
func (liveFollowRosterDomain LiveFollowRosterDomain) Symbols() []string {
	symbols := make([]string, 0, len(liveFollowRosterDomain.marketsBySymbol))
	for symbol := range liveFollowRosterDomain.marketsBySymbol {
		symbols = append(symbols, symbol)
	}

	return symbols
}

// Channels cuts each market's holders into channels of its allowed size, in a deterministic order so a channel can be recognised by what it carries.
func (liveFollowRosterDomain LiveFollowRosterDomain) Channels() []vo.LiveFollowChannelVo {
	symbolsByMarket := make(map[vo.MarketVo][]string)
	for symbol, market := range liveFollowRosterDomain.marketsBySymbol {
		symbolsByMarket[market] = append(symbolsByMarket[market], symbol)
	}

	channels := make([]vo.LiveFollowChannelVo, 0, len(symbolsByMarket))
	for _, market := range slices.Sorted(maps.Keys(symbolsByMarket)) {
		symbols := symbolsByMarket[market]
		slices.Sort(symbols)

		perChannel := liveFollowRosterDomain.symbolsPerChannelByMarket[market]
		for start := 0; start < len(symbols); start += perChannel {
			channels = append(channels, vo.NewLiveFollowChannelVo(
				market, symbols[start:min(start+perChannel, len(symbols))]))
		}
	}

	return channels
}

// HasLiveUpdates is always true for on-demand markets and otherwise true only for rostered symbols; it checks rostering rather than the cap, since uncapped rostered markets still follow only watched symbols.
func (liveFollowRosterDomain LiveFollowRosterDomain) HasLiveUpdates(
	symbol string, marketDomain MarketDomain,
) bool {
	if !marketDomain.FollowsFixedRoster() {
		return true
	}

	return liveFollowRosterDomain.Holds(symbol)
}
