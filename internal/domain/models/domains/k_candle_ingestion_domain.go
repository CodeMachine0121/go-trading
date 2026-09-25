package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleIngestionDomain owns every "which candles" rule of automatic ingestion, so the periodic round and startup backfill share one path and differ only in window.
type KCandleIngestionDomain struct {
	currentTime      time.Time
	roundCandleCount int
	backfillLookback time.Duration
}

// NewKCandleIngestionDomain refuses non-positive settings and raises a lookback below one coarsest bucket to that bucket, since bucket alignment would otherwise far overshoot a tiny lookback.
func NewKCandleIngestionDomain(
	currentTime time.Time,
	roundCandleCount int,
	backfillLookback time.Duration,
) (KCandleIngestionDomain, error) {
	if roundCandleCount <= 0 {
		return KCandleIngestionDomain{}, fmt.Errorf(
			"%w: 單輪取回根數必須大於零", ErrKCandleIngestionValidation)
	}

	if backfillLookback <= 0 {
		return KCandleIngestionDomain{}, fmt.Errorf(
			"%w: 回補上限必須大於零", ErrKCandleIngestionValidation)
	}

	return KCandleIngestionDomain{
		currentTime:      currentTime.UTC(),
		roundCandleCount: roundCandleCount,
		backfillLookback: max(backfillLookback, coarsestBucketSpan()),
	}, nil
}

// coarsestBucketSpan is read off the interval set so it cannot drift when a coarser interval is added.
func coarsestBucketSpan() time.Duration {
	return time.Duration(
		NewCoarsestAggregationIntervalDomain().SourceCandleCount(1)) * KCandleInterval
}

// CurrentTime is exposed so fetched candles are judged "in the future" against the same moment the window was built from.
func (kCandleIngestionDomain KCandleIngestionDomain) CurrentTime() time.Time {
	return kCandleIngestionDomain.currentTime
}

// LatestClosedOpenTime is the newest candle whose interval has fully elapsed.
func (kCandleIngestionDomain KCandleIngestionDomain) LatestClosedOpenTime() time.Time {
	interval := kCandleIngestionDomain.interval()

	return kCandleIngestionDomain.currentTime.Truncate(interval).Add(-interval)
}

// ScheduledWindow covers the newest closed candle and a few before it to absorb source corrections and refill failed rounds; market hours are narrowed afterwards by the market.
func (kCandleIngestionDomain KCandleIngestionDomain) ScheduledWindow(
	symbol string, market vo.MarketVo,
) vo.KCandleFetchWindowVo {
	endTime := kCandleIngestionDomain.LatestClosedOpenTime()
	candlesBefore := time.Duration(kCandleIngestionDomain.roundCandleCount-1) *
		kCandleIngestionDomain.interval()

	return vo.NewKCandleFetchWindowVo(symbol, market, endTime.Add(-candlesBefore), endTime)
}

// BackfillWindow covers the gap since the latest stored candle (zero means never stored), or the whole lookback, and may come back empty.
// A lookback-derived start is rounded down to a coarsest-bucket edge so no bucket starts partway through; a stored-data start is left as is to avoid refetching.
func (kCandleIngestionDomain KCandleIngestionDomain) BackfillWindow(
	symbol string,
	market vo.MarketVo,
	latestStoredOpenTime time.Time,
) vo.KCandleFetchWindowVo {
	startTime := NewCoarsestAggregationIntervalDomain().BucketStart(
		kCandleIngestionDomain.currentTime.Add(-kCandleIngestionDomain.backfillLookback))

	if !latestStoredOpenTime.IsZero() {
		nextAfterStored := latestStoredOpenTime.UTC().Add(kCandleIngestionDomain.interval())
		if nextAfterStored.After(startTime) {
			startTime = nextAfterStored
		}
	}

	return vo.NewKCandleFetchWindowVo(
		symbol, market, startTime, kCandleIngestionDomain.LatestClosedOpenTime())
}

// HistoryWindow ignores stored data so holes in the middle of history get refetched, which a backfill never does; its start is bucket-aligned like BackfillWindow's.
func (kCandleIngestionDomain KCandleIngestionDomain) HistoryWindow(
	symbol string, market vo.MarketVo, lookback time.Duration,
) vo.KCandleFetchWindowVo {
	startTime := NewCoarsestAggregationIntervalDomain().BucketStart(
		kCandleIngestionDomain.currentTime.Add(-lookback))

	return vo.NewKCandleFetchWindowVo(
		symbol, market, startTime, kCandleIngestionDomain.LatestClosedOpenTime())
}

// HistoryChunks splits HistoryWindow into contiguous one-day chunks, oldest first, bounding memory per chunk and leaving any interruption as a recent-end gap the backfill closes.
func (kCandleIngestionDomain KCandleIngestionDomain) HistoryChunks(
	symbol string, market vo.MarketVo, lookback time.Duration,
) []vo.KCandleFetchWindowVo {
	wholeWindow := kCandleIngestionDomain.HistoryWindow(symbol, market, lookback)
	if wholeWindow.IsEmpty() {
		return []vo.KCandleFetchWindowVo{}
	}

	coarsestInterval := NewCoarsestAggregationIntervalDomain()

	chunks := make([]vo.KCandleFetchWindowVo, 0)
	for chunkStart := wholeWindow.StartTime; !chunkStart.After(wholeWindow.EndTime); {
		chunkEnd := coarsestInterval.BucketStart(chunkStart).
			Add(coarsestInterval.duration).Add(-kCandleIngestionDomain.interval())
		if chunkEnd.After(wholeWindow.EndTime) {
			chunkEnd = wholeWindow.EndTime
		}

		chunks = append(chunks,
			vo.NewKCandleFetchWindowVo(symbol, market, chunkStart, chunkEnd))

		chunkStart = chunkEnd.Add(kCandleIngestionDomain.interval())
	}

	return chunks
}

// SelectClosed drops any candle whose interval has not finished, however the source reported it.
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

// RoundCoverage is the time one scheduled round asks about, needed to judge whether an empty answer means anything.
func (kCandleIngestionDomain KCandleIngestionDomain) RoundCoverage() time.Duration {
	return time.Duration(kCandleIngestionDomain.roundCandleCount) * kCandleIngestionDomain.interval()
}

func (kCandleIngestionDomain KCandleIngestionDomain) interval() time.Duration {
	return KCandleInterval
}
