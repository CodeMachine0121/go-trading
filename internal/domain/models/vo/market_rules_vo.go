package vo

import "time"

// TradingSessionVo is the stretch of each day a market trades in, said in that
// market's own zone rather than universal time — because "the Taiwan market opens at
// nine" is a fact about Taipei, and the moment it names in universal time is not even
// the same one all year in markets that shift with daylight saving.
//
// A nil Location means the market never closes, which is how a round-the-clock market
// is expressed rather than a flag somewhere else. It makes "always open" a value the
// same rules read, instead of a branch every reader has to remember.
//
// Immutable, no behavior — asking whether a moment falls inside one is MarketDomain's.
type TradingSessionVo struct {
	// Location is the zone the daily times are said in. Nil means never closes.
	Location *time.Location
	// DailyStart is how far into the local day trading begins.
	DailyStart time.Duration
	// DailyEnd is how far into the local day trading ends. The last K candle of the
	// day is the one that finishes exactly here, so it opens one candle length before.
	DailyEnd time.Duration
	// Weekdays are the days of the week trading happens at all. Empty means none,
	// which is only sensible for a market that never closes and therefore never
	// consults this.
	Weekdays []time.Weekday
}

// MarketRulesVo is everything that differs between one market and the next: when it
// trades, and what its market data plan allows of a live feed.
//
// They live together because they are the same kind of fact — a property of the
// venue, settled outside this system, and changed by a setting rather than by code.
// A third market is a third entry, not a third branch.
//
// The two live-feed numbers are the two the plans are actually written in, and how
// many symbols may be followed at once is worked out from them rather than kept
// alongside as a third. Kept alongside, somebody could set a ceiling of five against
// a plan that allows one line of one symbol — which is a state nothing could detect
// and everything downstream would believe.
//
// Immutable, no behavior — MarketDomain is where these become answers.
type MarketRulesVo struct {
	TradingSession TradingSessionVo
	// SimultaneousChannelCeiling is how many live channels this market's plan allows
	// to be open at the same time. Zero means no ceiling.
	SimultaneousChannelCeiling int
	// SymbolsPerLiveChannel is how many trading symbols one of this market's live
	// channels may carry. Zero means one — a channel always carries at least one
	// symbol, and a market whose source follows them one at a time is then written
	// as the zero value rather than as a number somebody has to remember to set.
	SymbolsPerLiveChannel int
}
