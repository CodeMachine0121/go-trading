package vo

// BacktestRunStatusVo is where one backtest run has got to.
type BacktestRunStatusVo string

const (
	// BacktestRunQueued is written when the request is accepted; no replica has taken it yet.
	BacktestRunQueued    BacktestRunStatusVo = "queued"
	BacktestRunRunning   BacktestRunStatusVo = "running"
	BacktestRunSucceeded BacktestRunStatusVo = "succeeded"
	// BacktestRunFailed covers script failures, the allowance running out, restarts and storage errors.
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
