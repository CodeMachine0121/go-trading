package dto

// The four states a viewer can observe while following a market. They are the
// whole vocabulary of this feature's output: anything a viewer sees is one of
// these, and nothing else is reported.
const (
	// KCandleFollowStatusForming is the candle currently being built. Its figures
	// will still move, so it is shown and never stored.
	KCandleFollowStatusForming = "forming"
	// KCandleFollowStatusClosed is a candle's last word: the interval it covers has
	// finished, so this is what gets stored.
	KCandleFollowStatusClosed = "closed"
	// KCandleFollowStatusStalled says live updating has stopped. It is an update
	// rather than an error because "the picture froze" is news a viewer must
	// receive, not a request that failed.
	KCandleFollowStatusStalled = "stalled"
	// KCandleFollowStatusUnavailable says this market has no live updating to give,
	// and will not have any without something changing.
	//
	// It is deliberately not the same word as stalled. Stalled means "wait, it is
	// coming back"; this means "it is not coming back, and waiting is the wrong
	// thing to do" — so a viewer told the wrong one of the two waits for something
	// that will never arrive.
	KCandleFollowStatusUnavailable = "unavailable"
)

// KCandleFollowUpdateDto is the only shape a viewer receives while following a
// market. KCandle is the zero value when Status is stalled — there is no candle to
// report, only the fact that there will not be one for now.
type KCandleFollowUpdateDto struct {
	Symbol  string     `json:"symbol"`
	Status  string     `json:"status"`
	KCandle KCandleDto `json:"kCandle"`
}
