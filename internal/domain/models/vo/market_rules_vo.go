package vo

import "time"

// TradingSessionVo is a market's daily trading window in its own time zone (so daylight saving is handled); a nil Location means it never closes.
type TradingSessionVo struct {
	Location   *time.Location
	DailyStart time.Duration
	// DailyEnd is when the day's last K candle finishes.
	DailyEnd time.Duration
	// Weekdays empty means none, which only makes sense for a market that never closes.
	Weekdays []time.Weekday
}

// MarketRulesVo is everything that differs between markets; the follow ceiling is derived from the two plan numbers rather than stored, so it can never contradict them.
type MarketRulesVo struct {
	TradingSession TradingSessionVo
	// FollowsFixedRoster means the market follows every watched symbol from a roster instead of on demand; it is set explicitly, not inferred from the ceiling.
	FollowsFixedRoster bool
	// SimultaneousChannelCeiling is how many live channels may be open at once; zero means no ceiling.
	SimultaneousChannelCeiling int
	// SymbolsPerLiveChannel is how many symbols one channel may carry; zero means one.
	SymbolsPerLiveChannel int
}
