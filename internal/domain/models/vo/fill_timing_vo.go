package vo

// FillTimingVo is at what price a replay fills a signal; see BacktestFillTimingDomain.
type FillTimingVo string

const (
	// FillTimingClose fills at the signalling bar's close, the original behaviour.
	FillTimingClose FillTimingVo = "close"
	// FillTimingNextOpen fills at the next bar's open, since the close has already passed when anyone can act.
	FillTimingNextOpen FillTimingVo = "nextOpen"
)
