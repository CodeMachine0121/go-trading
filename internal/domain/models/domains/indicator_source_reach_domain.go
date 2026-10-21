package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// IndicatorSourceReachDomain is how far back an indicator calculation reads: stretch by stretch, each ending on a bucket edge, until the latest candleCount buckets holding candles are found or history runs out.
// It finds exactly the buckets a count-limited read of the latest minutes would keep, since a bucket holds at most its minute capacity and only the earliest one read could be cut short.
type IndicatorSourceReachDomain struct {
	symbol      string
	interval    AggregationIntervalDomain
	candleCount int
	cutoff      time.Time
	// stretchStart and stretchEnd bound the stretch to read next, end excluded, both on bucket edges.
	stretchStart            time.Time
	stretchEnd              time.Time
	foundBucketCount        int
	stretchFoundBucketCount int
}

// ReadsSummaries is false for intervals with no stored summaries; those keep reading the latest minutes by count.
func (reachDomain IndicatorSourceReachDomain) ReadsSummaries() bool {
	_, hasSummaries := reachDomain.interval.summaryInterval()

	return hasSummaries
}

func (reachDomain IndicatorSourceReachDomain) Symbol() string {
	return reachDomain.symbol
}

// Cutoff is the start of the bucket holding the end time; nothing opened at or after it is read.
func (reachDomain IndicatorSourceReachDomain) Cutoff() time.Time {
	return reachDomain.cutoff
}

// SourceCandleLimit is the minute count read when summaries are not used: the requested buckets plus one spare.
func (reachDomain IndicatorSourceReachDomain) SourceCandleLimit() int {
	return reachDomain.interval.SourceCandleCount(reachDomain.candleCount + spareBucketCount)
}

// SeriesQuery is the current stretch in the shape the summarized series reader takes.
func (reachDomain IndicatorSourceReachDomain) SeriesQuery() KCandleSeriesQueryDomain {
	return KCandleSeriesQueryDomain{
		rangeQuery: KCandleQueryDomain{
			symbol:    reachDomain.symbol,
			startTime: reachDomain.stretchStart,
			endTime:   reachDomain.stretchEnd.Add(-time.Microsecond),
		},
		interval:    reachDomain.interval,
		bucketCount: int(reachDomain.stretchEnd.Sub(reachDomain.stretchStart) / reachDomain.interval.duration),
	}
}

// CountingKCandles adds the buckets the current stretch's spot candles fill.
func (reachDomain IndicatorSourceReachDomain) CountingKCandles(kCandles []entities.KCandle) IndicatorSourceReachDomain {
	openTimes := make([]time.Time, 0, len(kCandles))
	for _, kCandle := range kCandles {
		openTimes = append(openTimes, kCandle.OpenTime)
	}

	return reachDomain.counting(openTimes)
}

// CountingKCandleContracts adds the buckets the current stretch's contract candles fill.
func (reachDomain IndicatorSourceReachDomain) CountingKCandleContracts(
	kCandleContracts []entities.KCandleContract,
) IndicatorSourceReachDomain {
	openTimes := make([]time.Time, 0, len(kCandleContracts))
	for _, kCandleContract := range kCandleContracts {
		openTimes = append(openTimes, kCandleContract.OpenTime)
	}

	return reachDomain.counting(openTimes)
}

func (reachDomain IndicatorSourceReachDomain) counting(openTimes []time.Time) IndicatorSourceReachDomain {
	bucketStarts := make(map[time.Time]bool, len(openTimes))
	for _, openTime := range openTimes {
		bucketStarts[reachDomain.interval.BucketStart(openTime)] = true
	}

	reachDomain.foundBucketCount += len(bucketStarts)
	reachDomain.stretchFoundBucketCount = len(bucketStarts)

	return reachDomain
}

func (reachDomain IndicatorSourceReachDomain) IsCovered() bool {
	return reachDomain.foundBucketCount >= reachDomain.candleCount
}

// EarlierCutoff is where to look for the latest candle older than everything read so far.
func (reachDomain IndicatorSourceReachDomain) EarlierCutoff() time.Time {
	return reachDomain.stretchStart
}

// ReachingBackTo jumps over the empty stretch to the bucket holding the earlier candle, sized by how densely the last stretch held buckets so closed markets and halts need few stretches.
// The length only affects how much is read; extra buckets are dropped by the selection, and each step finds at least the earlier candle's bucket, so the reach always ends.
func (reachDomain IndicatorSourceReachDomain) ReachingBackTo(earlierOpenTime time.Time) IndicatorSourceReachDomain {
	remainingBucketCount := reachDomain.candleCount - reachDomain.foundBucketCount
	stretchBucketCount := int(reachDomain.stretchEnd.Sub(reachDomain.stretchStart) / reachDomain.interval.duration)
	nextStretchBucketCount := remainingBucketCount
	if reachDomain.stretchFoundBucketCount > 0 {
		nextStretchBucketCount = max(remainingBucketCount,
			(remainingBucketCount*stretchBucketCount+reachDomain.stretchFoundBucketCount-1)/
				reachDomain.stretchFoundBucketCount)
	}

	reachDomain.stretchEnd = reachDomain.interval.BucketStart(earlierOpenTime).Add(reachDomain.interval.duration)
	reachDomain.stretchStart = reachDomain.stretchEnd.Add(
		-time.Duration(nextStretchBucketCount) * reachDomain.interval.duration)

	return reachDomain
}
