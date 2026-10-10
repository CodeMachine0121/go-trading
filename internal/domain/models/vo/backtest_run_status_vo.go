package vo

type BacktestRunStatusVo string

const (
	BacktestRunQueued    BacktestRunStatusVo = "queued"
	BacktestRunRunning   BacktestRunStatusVo = "running"
	BacktestRunSucceeded BacktestRunStatusVo = "succeeded"
	BacktestRunFailed    BacktestRunStatusVo = "failed"
	BacktestRunCancelled BacktestRunStatusVo = "cancelled"
)

// NewBacktestRunStatusVo reads a stored status; anything unrecognised reads as failed, whose worst case is a re-run.
func NewBacktestRunStatusVo(status string) BacktestRunStatusVo {
	switch BacktestRunStatusVo(status) {
	case BacktestRunQueued, BacktestRunRunning, BacktestRunSucceeded, BacktestRunCancelled:
		return BacktestRunStatusVo(status)
	default:
		return BacktestRunFailed
	}
}
