package dto

const (
	// KCandleFollowStatusForming is the in-progress candle; shown but never stored.
	KCandleFollowStatusForming = "forming"
	// KCandleFollowStatusClosed is a finished candle and is what gets stored.
	KCandleFollowStatusClosed = "closed"
	// KCandleFollowStatusStalled means live updates stopped temporarily and are expected back.
	KCandleFollowStatusStalled = "stalled"
	// KCandleFollowStatusUnavailable means the market has no live updates and waiting will
	// not help.
	KCandleFollowStatusUnavailable = "unavailable"
	// KCandleFollowStatusMarketClosed means updates resume on their own when the market reopens.
	KCandleFollowStatusMarketClosed = "marketClosed"
)

// KCandleFollowUpdateDto has a zero KCandle when Status is stalled.
type KCandleFollowUpdateDto struct {
	Symbol  string     `json:"symbol"`
	Status  string     `json:"status"`
	KCandle KCandleDto `json:"kCandle"`
}
