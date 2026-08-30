package vo

import "time"

// KCandleFetchWindowVo is one stretch of time to fetch K candles for. It is the
// single shape both the periodic round and the startup backfill hand to a market
// source, which is why the source needs only one way to be asked.
type KCandleFetchWindowVo struct {
	Symbol    string
	StartTime time.Time
	EndTime   time.Time
}

// NewKCandleFetchWindowVo pins the window to universal time, whatever zone the
// caller worked in.
func NewKCandleFetchWindowVo(symbol string, startTime time.Time, endTime time.Time) KCandleFetchWindowVo {
	return KCandleFetchWindowVo{
		Symbol:    symbol,
		StartTime: startTime.UTC(),
		EndTime:   endTime.UTC(),
	}
}

// IsEmpty reports a window that covers no K candle at all. A backfill with no gap
// to fill produces one, which turns "there is nothing to do" into a value the
// caller can read rather than a branch it has to remember.
func (kCandleFetchWindowVo KCandleFetchWindowVo) IsEmpty() bool {
	return kCandleFetchWindowVo.StartTime.After(kCandleFetchWindowVo.EndTime)
}
