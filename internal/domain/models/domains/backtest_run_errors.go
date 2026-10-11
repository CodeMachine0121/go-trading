package domains

import (
	"errors"
	"fmt"
)

var ErrBacktestRunNotFound = errors.New("backtest run not found")

// ErrBacktestRunBrokeDown stands for a replay that panicked, so its run ends without exposing the panic.
var ErrBacktestRunBrokeDown = errors.New("backtest run broke down")

var ErrBacktestRunAlreadyEnded = errors.New("這筆回測已經結束，不能取消")

var ErrBacktestRunOwnerLimitReached = errors.New("backtest run owner limit reached")

type backtestRunOwnerLimitError struct {
	limit int
}

func (ownerLimitError *backtestRunOwnerLimitError) Error() string {
	return fmt.Sprintf("你同時最多 %d 筆回測在排隊或進行中，等其中一筆結束再送", ownerLimitError.limit)
}

func (ownerLimitError *backtestRunOwnerLimitError) Unwrap() error {
	return ErrBacktestRunOwnerLimitReached
}

func BacktestRunOwnerLimit(err error) (int, bool) {
	var ownerLimitError *backtestRunOwnerLimitError
	if !errors.As(err, &ownerLimitError) {
		return 0, false
	}

	return ownerLimitError.limit, true
}
