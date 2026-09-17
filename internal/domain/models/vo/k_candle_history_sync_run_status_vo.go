package vo

// KCandleHistorySyncRunStatusVo is where one history sync has got to.
//
// There are three and only three. It is still fetching, it finished, or it stopped
// without finishing — and a reader who cannot tell those apart cannot tell a run
// still working through four years from one that gave up in the first minute.
type KCandleHistorySyncRunStatusVo string

const (
	// KCandleHistorySyncRunning is recorded the moment the request is accepted,
	// before the source has been asked anything. It is what makes the work findable
	// while it is still going, which is the whole reason the row is written up front.
	KCandleHistorySyncRunning KCandleHistorySyncRunStatusVo = "running"
	// KCandleHistorySyncSucceeded is the whole stretch walked to the end. It does not
	// promise every candle arrived — a source that answered nothing for a chunk still
	// answered — only that nothing stopped the run.
	KCandleHistorySyncSucceeded KCandleHistorySyncRunStatusVo = "succeeded"
	// KCandleHistorySyncFailed is every way a run ends early: storage breaking, or
	// the system being restarted while it was still fetching.
	KCandleHistorySyncFailed KCandleHistorySyncRunStatusVo = "failed"
)

// NewKCandleHistorySyncRunStatusVo reads a stored status.
//
// **Anything it does not understand reads as failed.** Of the three, failed is the
// one whose worst case is somebody running the sync again. Read as running it would
// be a wait that never ends; read as succeeded it would claim a stretch is held that
// may never have been fetched.
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
