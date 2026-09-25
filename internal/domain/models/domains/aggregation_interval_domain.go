package domains

import (
	"fmt"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// selectableAggregationInterval keeps each interval and its length together so neither can be declared without the other.
type selectableAggregationInterval struct {
	value    vo.AggregationIntervalVo
	duration time.Duration
}

// selectableAggregationIntervals is ordered shortest first (also the order offered in error messages), and every length must divide a day exactly.
// Buckets are cut from UTC midnight and backfill aligns to the coarsest bucket edge, both of which rely on that invariant.
var selectableAggregationIntervals = []selectableAggregationInterval{
	{value: vo.AggregationIntervalOneMinute, duration: KCandleInterval},
	{value: vo.AggregationIntervalFiveMinutes, duration: 5 * time.Minute},
	{value: vo.AggregationIntervalFifteenMinutes, duration: 15 * time.Minute},
	{value: vo.AggregationIntervalOneHour, duration: time.Hour},
	{value: vo.AggregationIntervalFourHours, duration: 4 * time.Hour},
	{value: vo.AggregationIntervalOneDay, duration: 24 * time.Hour},
}

// AggregationIntervalDomain is one declared interval; its zero value is unusable and only returned with an error.
type AggregationIntervalDomain struct {
	value    vo.AggregationIntervalVo
	duration time.Duration
}

// NewAggregationIntervalDomain defaults an empty declaration to one minute and matches case-insensitively.
// The error has no sentinel of its own so each caller can wrap it in its own validation error.
func NewAggregationIntervalDomain(declared string) (AggregationIntervalDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declared)
	if normalizedDeclaration == "" {
		return newAggregationIntervalDomain(selectableAggregationIntervals[0]), nil
	}

	for _, selectableInterval := range selectableAggregationIntervals {
		if strings.EqualFold(string(selectableInterval.value), normalizedDeclaration) {
			return newAggregationIntervalDomain(selectableInterval), nil
		}
	}

	selectableSpellings := make([]string, 0, len(selectableAggregationIntervals))
	for _, selectableInterval := range selectableAggregationIntervals {
		selectableSpellings = append(selectableSpellings, string(selectableInterval.value))
	}

	return AggregationIntervalDomain{}, fmt.Errorf(
		"彙總刻度只能是 %s 其中之一", strings.Join(selectableSpellings, "、"))
}

// NewCoarsestAggregationIntervalDomain is the interval to align bucket edges to, since its edges are a subset of every other interval's.
// It compares lengths instead of trusting the list order.
func NewCoarsestAggregationIntervalDomain() AggregationIntervalDomain {
	return newAggregationIntervalDomain(coarsestSelectableAggregationInterval())
}

// NewFittingAggregationIntervalDomain returns the finest interval whose trading bucket count fits displayableCandleCount, falling back to the coarsest.
// Buckets are counted over the market's sessions rather than by dividing the duration, since a closed market does not tick evenly.
func NewFittingAggregationIntervalDomain(
	marketDomain MarketDomain,
	startTime time.Time,
	endTime time.Time,
	displayableCandleCount int,
) AggregationIntervalDomain {
	fittingInterval := coarsestSelectableAggregationInterval()
	for _, selectableInterval := range selectableAggregationIntervals {
		candidate := newAggregationIntervalDomain(selectableInterval)
		if candidate.TradingSlotCount(marketDomain, startTime, endTime) > displayableCandleCount {
			continue
		}

		if selectableInterval.duration < fittingInterval.duration {
			fittingInterval = selectableInterval
		}
	}

	return newAggregationIntervalDomain(fittingInterval)
}

// coarsestSelectableAggregationInterval compares lengths rather than trusting the list order.
func coarsestSelectableAggregationInterval() selectableAggregationInterval {
	coarsestInterval := selectableAggregationIntervals[0]
	for _, selectableInterval := range selectableAggregationIntervals {
		if selectableInterval.duration > coarsestInterval.duration {
			coarsestInterval = selectableInterval
		}
	}

	return coarsestInterval
}

// newAggregationIntervalDomain is the only constructor, so an interval never exists without its length.
func newAggregationIntervalDomain(selectableInterval selectableAggregationInterval) AggregationIntervalDomain {
	return AggregationIntervalDomain{
		value:    selectableInterval.value,
		duration: selectableInterval.duration,
	}
}

func (aggregationIntervalDomain AggregationIntervalDomain) Value() vo.AggregationIntervalVo {
	return aggregationIntervalDomain.value
}

func (aggregationIntervalDomain AggregationIntervalDomain) BucketStart(moment time.Time) time.Time {
	return bucketStartOf(moment, aggregationIntervalDomain.duration)
}

// bucketStartOf cuts buckets from UTC midnight so a moment always lands in the same bucket; intervals and markets share it to keep that rule in one place.
func bucketStartOf(moment time.Time, bucketDuration time.Duration) time.Time {
	return moment.UTC().Truncate(bucketDuration)
}

// BucketCount includes both ends, so a range within one bucket counts as one.
func (aggregationIntervalDomain AggregationIntervalDomain) BucketCount(
	startTime time.Time, endTime time.Time,
) int {
	firstBucketStart := aggregationIntervalDomain.BucketStart(startTime)
	lastBucketStart := aggregationIntervalDomain.BucketStart(endTime)

	return int(lastBucketStart.Sub(firstBucketStart)/aggregationIntervalDomain.duration) + 1
}

// TradingSlotCount asks the market to count buckets over its sessions, since a bucket catching one minute of trading is a whole candle (a Taiwan day is five hourly buckets, not four).
// The floor of one is load-bearing: series queries over a closed stretch must still get an empty series rather than a refusal.
func (aggregationIntervalDomain AggregationIntervalDomain) TradingSlotCount(
	marketDomain MarketDomain, startTime time.Time, endTime time.Time,
) int {
	return max(1, marketDomain.TradingBucketCountBetween(
		startTime, endTime, aggregationIntervalDomain.duration))
}

// SourceCandleCount is the upper bound of stored K candles the buckets can hold, used as a read limit.
func (aggregationIntervalDomain AggregationIntervalDomain) SourceCandleCount(bucketCount int) int {
	candlesPerBucket := int(aggregationIntervalDomain.duration / KCandleInterval)

	return bucketCount * candlesPerBucket
}
