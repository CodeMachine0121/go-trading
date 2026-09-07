package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// LiveFollowRosterDomain is which symbols hold the live places of every market that
// limits how many may be followed at once.
//
// It exists because two very different questions have the same answer: what the
// system should be following right now, and what the console should say about a
// symbol before anybody picks it. Answering them in two places would let a chart
// promise live updates that nothing was following, or refuse them for something that
// was — and either way nobody would find out until they were watching a picture that
// did not move.
//
// A market with no ceiling holds no places here. Its follows are started by whoever
// looks at them, so there is no roster to be on; that is the difference between "no
// place for you" and "nobody has asked yet", and it is why the two are asked about
// separately.
type LiveFollowRosterDomain struct {
	// The market each holder belongs to is kept alongside, because whoever acts on a
	// place needs to know which venue to open a feed against — and looking it up
	// again is a second chance to get a different answer.
	marketsBySymbol map[string]vo.MarketVo
}

// NewLiveFollowRosterDomain hands each limited market's places to its
// earliest-registered watched symbols, in the order the watchlist arrives in.
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

	for _, watchedSymbol := range watchedSymbols {
		marketDomain := marketCatalogDomain.MarketOf(watchedSymbol.Market)
		if !marketDomain.IsOpen(currentTime) {
			continue
		}

		remaining, hasCounted := placesLeft[marketDomain.Value()]
		if !hasCounted {
			remaining = marketDomain.SimultaneousFollowCeiling()
		}

		// A market with no ceiling starts with no places, which is the same answer
		// arrived at by the same route as a market that has run out of them: there is
		// nothing here to give this symbol. Saying it twice — once as "no ceiling" and
		// once as "none left" — would be two spellings of one rule.
		if remaining <= 0 {
			placesLeft[marketDomain.Value()] = 0

			continue
		}

		marketsBySymbol[watchedSymbol.Symbol] = marketDomain.Value()
		placesLeft[marketDomain.Value()] = remaining - 1
	}

	return LiveFollowRosterDomain{marketsBySymbol: marketsBySymbol}
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

// HasLiveUpdates reports whether a symbol of this market can be followed live right
// now, which is the question a console asks before somebody picks it.
//
// A market with no ceiling always can: nothing is following it until somebody looks,
// but looking is all it takes. A limited market can only for the symbols holding its
// places — and telling somebody otherwise would hand them a chart that looks live and
// never moves.
func (liveFollowRosterDomain LiveFollowRosterDomain) HasLiveUpdates(
	symbol string, marketDomain MarketDomain,
) bool {
	if !marketDomain.HasFollowCeiling() {
		return true
	}

	return liveFollowRosterDomain.Holds(symbol)
}
