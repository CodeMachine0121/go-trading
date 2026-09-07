package dto

// TradingSymbolDto is the only shape in which a tradable symbol leaves the domain.
// It is an object rather than a bare name because the next thing anyone will ask for
// is what else we know about it — how many candles, how recent the latest one is —
// and adding a field is a compatible change while turning names into objects is not.
type TradingSymbolDto struct {
	Symbol string `json:"symbol"`
	// Market is which market this symbol belongs to.
	Market string `json:"market"`
	// IsWatched says the system is keeping this symbol's candles up to date.
	IsWatched bool `json:"isWatched"`
	// IsWithinTradingSession says its market is trading at this moment.
	//
	// It travels with the symbol rather than being worked out by whoever displays it,
	// because a console cannot know which days a market takes off. Working it out
	// from the clock alone would call a public holiday an outage.
	IsWithinTradingSession bool `json:"isWithinTradingSession"`
	// HasLiveUpdates says this symbol can be followed live right now.
	//
	// It also travels with the symbol, and for the same reason: which symbols hold a
	// limited market's live places is decided here, so anywhere else it could only be
	// guessed — and a guess shows somebody a chart that promises to move and does not.
	HasLiveUpdates bool `json:"hasLiveUpdates"`
}
