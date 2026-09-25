package vo

// KCandleHistorySyncRunStatusVo is where one history sync has got to: running, succeeded, or failed.
type KCandleHistorySyncRunStatusVo string

const (
	// KCandleHistorySyncRunning is written up front so the run is findable while it is still going.
	KCandleHistorySyncRunning KCandleHistorySyncRunStatusVo = "running"
	// KCandleHistorySyncSucceeded means nothing stopped the run, not that every candle arrived.
	KCandleHistorySyncSucceeded KCandleHistorySyncRunStatusVo = "succeeded"
	// KCandleHistorySyncFailed covers storage errors and restarts mid-fetch.
	KCandleHistorySyncFailed KCandleHistorySyncRunStatusVo = "failed"
)

// NewKCandleHistorySyncRunStatusVo reads a stored status; anything unrecognised reads as failed, whose worst case is a re-run.
func NewKCandleHistorySyncRunStatusVo(status string) KCandleHistorySyncRunStatusVo {
	switch KCandleHistorySyncRunStatusVo(status) {
	case KCandleHistorySyncRunning:
		return KCandleHistorySyncRunning
	case KCandleHistorySyncSucceeded:
		return KCandleHistorySyncSucceeded
	default:
		return KCandleHistorySyncFailed
	}
}
