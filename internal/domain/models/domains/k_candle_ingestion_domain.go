package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleIngestionDomain owns every "which candles" rule of automatic ingestion:
// which candle counts as closed, and the two stretches of time worth fetching.
//
// The periodic round and the startup backfill differ only in the window they ask
// for; everything after that is one shared path. Keeping both window rules on one
// object is what makes that true, and it is why widening the K candle length later
// touches this file and no other.
type KCandleIngestionDomain struct {
	currentTime      time.Time
	roundCandleCount int
	backfillLookback time.Duration
}

// NewKCandleIngestionDomain judges "closed" and "how far back" against currentTime.
// It does not assume the caller already screened the candle count.
func NewKCandleIngestionDomain(
	currentTime time.Time,
	roundCandleCount int,
	backfillLookback time.Duration,
) (KCandleIngestionDomain, error) {
	if roundCandleCount <= 0 {
		return KCandleIngestionDomain{}, fmt.Errorf(
			"%w: 單輪取回根數必須大於零", ErrKCandleIngestionValidation)
	}

	return KCandleIngestionDomain{
		currentTime:      currentTime.UTC(),
		roundCandleCount: roundCandleCount,
		backfillLookback: backfillLookback,
	}, nil
}

// CurrentTime is the moment every ingestion rule is judged against. It is handed
// out so that a caller applying the K candle rules to a fetched candle judges
// "in the future" against the same moment the window was built from.
func (kCandleIngestionDomain KCandleIngestionDomain) CurrentTime() time.Time {
	return kCandleIngestionDomain.currentTime
}

// LatestClosedOpenTime is the open time of the newest K candle whose interval has
// fully elapsed. The one after it is still running, so its figures would still move.
func (kCandleIngestionDomain KCandleIngestionDomain) LatestClosedOpenTime() time.Time {
	interval := kCandleIngestionDomain.interval()

	return kCandleIngestionDomain.currentTime.Truncate(interval).Add(-interval)
}

// ScheduledWindow covers the newest closed candle and the few before it. More than
// one is deliberate: it absorbs figures the source corrects after the fact, and it
// quietly refills whatever a failed round left behind.
func (kCandleIngestionDomain KCandleIngestionDomain) ScheduledWindow(symbol string) vo.KCandleFetchWindowVo {
	endTime := kCandleIngestionDomain.LatestClosedOpenTime()
	candlesBefore := time.Duration(kCandleIngestionDomain.roundCandleCount-1) *
		kCandleIngestionDomain.interval()

	return vo.NewKCandleFetchWindowVo(symbol, endTime.Add(-candlesBefore), endTime)
}

// BackfillWindow covers the gap left behind while nothing was running, reaching no
// further back than the lookback allows. A zero latestStoredOpenTime means the
// symbol has never held a K candle, which fills the whole lookback. When the gap is
// already closed the window comes back empty.
func (kCandleIngestionDomain KCandleIngestionDomain) BackfillWindow(
	symbol string,
	latestStoredOpenTime time.Time,
) vo.KCandleFetchWindowVo {
	startTime := kCandleIngestionDomain.currentTime.Add(-kCandleIngestionDomain.backfillLookback)

	if !latestStoredOpenTime.IsZero() {
		nextAfterStored := latestStoredOpenTime.UTC().Add(kCandleIngestionDomain.interval())
		if nextAfterStored.After(startTime) {
			startTime = nextAfterStored
		}
	}

	return vo.NewKCandleFetchWindowVo(symbol, startTime, kCandleIngestionDomain.LatestClosedOpenTime())
}

// SelectClosed drops any candle the source handed over whose interval has not
// finished yet, however the source chose to report it.
func (kCandleIngestionDomain KCandleIngestionDomain) SelectClosed(
	marketKCandles []vo.MarketKCandleVo,
) []vo.MarketKCandleVo {
	latestClosedOpenTime := kCandleIngestionDomain.LatestClosedOpenTime()

	closedKCandles := make([]vo.MarketKCandleVo, 0, len(marketKCandles))
	for _, marketKCandle := range marketKCandles {
		if !marketKCandle.OpenTime.UTC().After(latestClosedOpenTime) {
			closedKCandles = append(closedKCandles, marketKCandle)
		}
	}

	return closedKCandles
}

// interval is how long one K candle covers, taken from the single place the
// project writes that length down.
func (kCandleIngestionDomain KCandleIngestionDomain) interval() time.Duration {
	return time.Duration(kCandleIntervalMinutes) * time.Minute
}
