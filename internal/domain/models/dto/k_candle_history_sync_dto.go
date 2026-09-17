package dto

// KCandleHistorySyncDto is one request to go and fetch a named stretch of a trading
// symbol's history.
//
// There is no coarseness here, and that absence is the design: the system stores one
// kind of K candle and computes every coarser one from it. A field for it would
// suggest there is another answer.
//
// LookbackDays has no default. Saying how far back is the whole point of this
// request, so a missing figure arrives as zero and is refused — rather than quietly
// standing in for the one thing the caller was choosing.
type KCandleHistorySyncDto struct {
	Symbol       string
	LookbackDays int
}
