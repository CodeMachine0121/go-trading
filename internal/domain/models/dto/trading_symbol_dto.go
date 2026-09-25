package dto

// TradingSymbolDto is an object rather than a bare name so fields can be added compatibly.
type TradingSymbolDto struct {
	Symbol string `json:"symbol"`
	// DisplayName is the venue's name (台積電 for 2330) and is empty when the market names
	// nothing or the symbol predates it.
	DisplayName string `json:"displayName"`
	Market      string `json:"market"`
	// IsWatched means the system keeps this symbol's candles up to date.
	IsWatched bool `json:"isWatched"`
	// IsWithinTradingSession is computed server-side because clients cannot know market holidays.
	IsWithinTradingSession bool `json:"isWithinTradingSession"`
	// HasTradingSession means the market keeps hours and therefore has gaps that waiting
	// cannot fill.
	HasTradingSession bool `json:"hasTradingSession"`
	// HasLiveUpdates is decided server-side because live slots on limited markets are
	// allocated here.
	HasLiveUpdates bool `json:"hasLiveUpdates"`
}
