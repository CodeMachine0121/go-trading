package vo

import "time"

// KCandleFetchWindowVo is one stretch of time to fetch K candles for; its Market field lets a routing source pick the right venue.
type KCandleFetchWindowVo struct {
	Symbol    string
	Market    MarketVo
	StartTime time.Time
	EndTime   time.Time
}

// NewKCandleFetchWindowVo normalizes both ends to UTC.
func NewKCandleFetchWindowVo(
	symbol string, market MarketVo, startTime time.Time, endTime time.Time,
) KCandleFetchWindowVo {
	return KCandleFetchWindowVo{
		Symbol:    symbol,
		Market:    market,
		StartTime: startTime.UTC(),
		EndTime:   endTime.UTC(),
	}
}

// IsEmpty reports a window that covers no K candle, e.g. a backfill with no gap.
func (kCandleFetchWindowVo KCandleFetchWindowVo) IsEmpty() bool {
	return kCandleFetchWindowVo.StartTime.After(kCandleFetchWindowVo.EndTime)
}
