package domains

import (
	"fmt"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// selectableAggregationInterval pairs a declarable interval with how long it covers.
// Splitting these into two structures once cost us both halves of the same mistake:
// declaring an interval without its length divides by zero at query time, and giving
// it a length without declaring it makes it unrecognisable. One list, one place.
type selectableAggregationInterval struct {
	value    vo.AggregationIntervalVo
	duration time.Duration
}

// selectableAggregationIntervals is the entire set a caller may declare, shortest
// first — that order is also the order they are offered back in when a declaration is
// not recognised. Every length here MUST divide a day exactly: bucket edges are cut
// from midnight in universal time, so a length that does not divide a day would drift
// a little further every day.
//
// Supporting one more interval means adding a row here and a constant in vo — nothing
// downstream branches per interval.
var selectableAggregationIntervals = []selectableAggregationInterval{
	{value: vo.AggregationIntervalFiveMinutes, duration: 5 * time.Minute},
	{value: vo.AggregationIntervalFifteenMinutes, duration: 15 * time.Minute},
	{value: vo.AggregationIntervalOneHour, duration: time.Hour},
	{value: vo.AggregationIntervalFourHours, duration: 4 * time.Hour},
	{value: vo.AggregationIntervalOneDay, duration: 24 * time.Hour},
}

// AggregationIntervalDomain is one declared aggregation interval and everything the
// rest of the system needs to know about it: how long it covers, which bucket a
// moment falls into, how many buckets a range is cut into, and how many stored K
// candles those buckets can possibly hold.
//
// Its zero value is not a usable interval; it is only ever returned alongside an error.
type AggregationIntervalDomain struct {
	value    vo.AggregationIntervalVo
	duration time.Duration
}

// NewAggregationIntervalDomain reads what the caller declared. Declaring nothing means
// the shortest interval — the length a stored K candle already covers — so a caller that knows
// nothing about aggregation gets exactly the candles it always got. Spelling is
// forgiving about surrounding blanks and letter case; anything else is refused, naming
// what could have been declared instead.
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
		"%w: 彙總刻度只能是 %s 其中之一",
		ErrKCandleValidation, strings.Join(selectableSpellings, "、"))
}

// newAggregationIntervalDomain is the only way an instance is built, so an interval
// can never exist without the length that goes with it.
func newAggregationIntervalDomain(selectableInterval selectableAggregationInterval) AggregationIntervalDomain {
	return AggregationIntervalDomain{
		value:    selectableInterval.value,
		duration: selectableInterval.duration,
	}
}

func (aggregationIntervalDomain AggregationIntervalDomain) Value() vo.AggregationIntervalVo {
	return aggregationIntervalDomain.value
}

// BucketStart is the start of the bucket the moment falls into. Buckets are cut from
// midnight in universal time, so the same moment always lands in the same bucket
// whatever range it was asked for as part of.
func (aggregationIntervalDomain AggregationIntervalDomain) BucketStart(moment time.Time) time.Time {
	return moment.UTC().Truncate(aggregationIntervalDomain.duration)
}

// BucketCount is how many buckets the range is cut into, both ends included. A range
// that starts and ends in the same bucket is one bucket, never zero.
func (aggregationIntervalDomain AggregationIntervalDomain) BucketCount(
	startTime time.Time, endTime time.Time,
) int {
	firstBucketStart := aggregationIntervalDomain.BucketStart(startTime)
	lastBucketStart := aggregationIntervalDomain.BucketStart(endTime)

	return int(lastBucketStart.Sub(firstBucketStart)/aggregationIntervalDomain.duration) + 1
}

// SourceCandleCount is the most stored K candles the given number of buckets can hold.
// One bucket holds as many candles as its length fits, and a trading symbol holds at
// most one candle per five-minute slot, so this is an upper bound the data cannot
// exceed — which is exactly what a read limit needs to be.
func (aggregationIntervalDomain AggregationIntervalDomain) SourceCandleCount(bucketCount int) int {
	candlesPerBucket := int(aggregationIntervalDomain.duration / (kCandleIntervalMinutes * time.Minute))

	return bucketCount * candlesPerBucket
}
