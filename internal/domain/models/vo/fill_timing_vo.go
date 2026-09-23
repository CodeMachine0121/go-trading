package vo

// FillTimingVo is at what price a replay fills a signal. Immutable, no behavior — how
// it is read lives in BacktestFillTimingDomain.
type FillTimingVo string

const (
	// FillTimingClose fills at the close of the bar that spoke — every replay made
	// before there was a choice.
	FillTimingClose FillTimingVo = "close"
	// FillTimingNextOpen fills at the open of the bar after the one that spoke: the
	// close had already happened by the time anyone could act on it.
	FillTimingNextOpen FillTimingVo = "nextOpen"
)
