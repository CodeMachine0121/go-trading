package domains

import (
	"errors"
	"fmt"
	"time"
)

// ErrKCandleHistoryLookback marks a lookback that is non-positive or beyond the configured ceiling.
var ErrKCandleHistoryLookback = errors.New("k candle history lookback rejected")

// ErrKCandleHistorySyncInProgress refuses rather than queues a second sync for the same symbol, since both would race over the same source allowance and rows.
var ErrKCandleHistorySyncInProgress = errors.New("k candle history sync already running")

func KCandleHistorySyncInProgress(symbol string) error {
	return fmt.Errorf(
		"%w: %s 已經有一趟歷史同步在跑了，等它結束再開下一趟",
		ErrKCandleHistorySyncInProgress, symbol)
}

// KCandleHistoryLookbackDomain is a history sync's reach in whole days, bounded by an operator-configured ceiling.
type KCandleHistoryLookbackDomain struct {
	days int
}

// NewKCandleHistoryLookbackDomain has no default because choosing how far back is the whole point of the request.
func NewKCandleHistoryLookbackDomain(
	days int, ceilingDays int,
) (KCandleHistoryLookbackDomain, error) {
	if days < 1 {
		return KCandleHistoryLookbackDomain{}, fmt.Errorf(
			"%w: 回溯天數必須至少 1 天，收到 %d", ErrKCandleHistoryLookback, days)
	}

	if days > ceilingDays {
		return KCandleHistoryLookbackDomain{}, fmt.Errorf(
			"%w: 回溯天數最多 %d 天，收到 %d", ErrKCandleHistoryLookback, ceilingDays, days)
	}

	return KCandleHistoryLookbackDomain{days: days}, nil
}

func (kCandleHistoryLookbackDomain KCandleHistoryLookbackDomain) Duration() time.Duration {
	return time.Duration(kCandleHistoryLookbackDomain.days) * 24 * time.Hour
}
