package domains

import (
	"maps"
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// LiveFollowRosterDomain is which symbols the system should be following live, for
// every market that is followed from a roster rather than by whoever looks.
//
// It exists because two very different questions have the same answer: what the
// system should be following right now, and what the console should say about a
// symbol before anybody picks it. Answering them in two places would let a chart
// promise live updates that nothing was following, or refuse them for something that
// was — and either way nobody would find out until they were watching a picture that
// did not move.
//
// A market whose follows begin when somebody opens a chart holds nothing here —
// there is no roster to be on; that is the difference between "no place for you" and
// "nobody has asked yet", and it is why the two are asked about separately.
//
// A rostered market may or may not cap how many places it hands out. Capped, the
// earliest-registered watched symbols take them. Uncapped, every watched symbol is
// on the roster — which is not the same as the roster being empty, and reading it as
// such would stop a market being followed at the exact moment its source stopped
// limiting it.
type LiveFollowRosterDomain struct {
	// The market each holder belongs to is kept alongside, because whoever acts on a
	// place needs to know which venue to open a feed against — and looking it up
	// again is a second chance to get a different answer.
	marketsBySymbol map[string]vo.MarketVo
	// How many symbols one of each market's channels carries, read once while the
	// catalogue was in hand. Asking the caller for it again when the channels are
	// wanted would be a second chance to answer it differently.
	symbolsPerChannelByMarket map[vo.MarketVo]int
}

// NewLiveFollowRosterDomain puts every watched symbol of a rostered market on the
// roster, in the order the watchlist arrives in — and where that market caps how
// many may be followed at once, stops at the cap.
//
// A market that is shut holds none. There is nothing to follow, and holding places
// open through the night would spend a limit that the morning needs.
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

		// Nothing is rostered for a market whose follows begin when somebody looks.
		// There is no place to give, and giving one would follow a symbol on behalf
		// of a viewer who never arrived.
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

// Holds reports whether this symbol is one the system should be following because it
// was given one of its market's places.
func (liveFollowRosterDomain LiveFollowRosterDomain) Holds(symbol string) bool {
	_, holdsAPlace := liveFollowRosterDomain.marketsBySymbol[symbol]

	return holdsAPlace
}

// MarketOf is the market a place holder belongs to, so that opening its feed needs
// nothing looked up again.
func (liveFollowRosterDomain LiveFollowRosterDomain) MarketOf(symbol string) vo.MarketVo {
	return liveFollowRosterDomain.marketsBySymbol[symbol]
}

// Symbols is every symbol holding a place, in no particular order — the places are
// held at the same time, so which of them came first buys nobody anything.
func (liveFollowRosterDomain LiveFollowRosterDomain) Symbols() []string {
	symbols := make([]string, 0, len(liveFollowRosterDomain.marketsBySymbol))
	for symbol := range liveFollowRosterDomain.marketsBySymbol {
		symbols = append(symbols, symbol)
	}

	return symbols
}

// Channels is what should be open right now: each market's place holders cut into
// channels of the size its plan allows one to carry.
//
// It is what the roster is for, rather than a convenience over Symbols. A caller
// given loose symbols would have to know how many go on a channel and cut them
// itself, and every caller that did would be a second place the plan is understood.
//
// The order is settled — markets by name, symbols within a channel sorted by the
// channel itself — so the same roster always produces the same channels, which is
// what lets a channel be recognised by what it carries.
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

// HasLiveUpdates reports whether a symbol of this market can be followed live right
// now, which is the question a console asks before somebody picks it.
//
// A market followed by whoever looks always can: nothing is following it until
// somebody does, but looking is all it takes. A rostered market can only for the
// symbols on its roster — and telling somebody otherwise would hand them a chart that
// looks live and never moves.
//
// It asks whether the market is rostered rather than whether it is capped, because
// an uncapped rostered market still follows only what is watched. Reading the cap
// here would promise live updates for every symbol of that market, including the ones
// nobody put on the watchlist and nothing is following.
func (liveFollowRosterDomain LiveFollowRosterDomain) HasLiveUpdates(
	symbol string, marketDomain MarketDomain,
) bool {
	if !marketDomain.FollowsFixedRoster() {
		return true
	}

	return liveFollowRosterDomain.Holds(symbol)
}
