package domains

import (
	"fmt"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// selectableAggregationIntervals is the entire set a caller may declare, in the order
// it is offered back when a declaration is not recognised. Every length here MUST
// divide a day exactly: bucket edges are cut from midnight in universal time, so a
// length that does not divide a day would drift a little further every day.
// Supporting one more interval means adding a row here and a constant in vo — nothing
// downstream branches per interval.
var selectableAggregationIntervals = map[vo.AggregationIntervalVo]time.Duration{
	vo.AggregationIntervalFiveMinutes:    5 * time.Minute,
	vo.AggregationIntervalFifteenMinutes: 15 * time.Minute,
	vo.AggregationIntervalOneHour:        time.Hour,
	vo.AggregationIntervalFourHours:      4 * time.Hour,
	vo.AggregationIntervalOneDay:         24 * time.Hour,
}

// selectableAggregationIntervalOrder fixes the order intervals are named in, shortest
// first. A map has no order of its own and an error message that reshuffles itself
// between runs is a bad error message.
var selectableAggregationIntervalOrder = []vo.AggregationIntervalVo{
	vo.AggregationIntervalFiveMinutes,
	vo.AggregationIntervalFifteenMinutes,
	vo.AggregationIntervalOneHour,
	vo.AggregationIntervalFourHours,
	vo.AggregationIntervalOneDay,
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
// five minutes — the length a stored K candle already covers — so a caller that knows
// nothing about aggregation gets exactly the candles it always got. Spelling is
// forgiving about surrounding blanks and letter case; anything else is refused, naming
// what could have been declared instead.
func NewAggregationIntervalDomain(declared string) (AggregationIntervalDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declared)
	if normalizedDeclaration == "" {
		return AggregationIntervalDomain{
			value:    vo.AggregationIntervalFiveMinutes,
			duration: selectableAggregationIntervals[vo.AggregationIntervalFiveMinutes],
		}, nil
	}

	for _, selectableInterval := range selectableAggregationIntervalOrder {
		if strings.EqualFold(string(selectableInterval), normalizedDeclaration) {
			return AggregationIntervalDomain{
				value:    selectableInterval,
				duration: selectableAggregationIntervals[selectableInterval],
			}, nil
		}
	}

	selectableSpellings := make([]string, 0, len(selectableAggregationIntervalOrder))
	for _, selectableInterval := range selectableAggregationIntervalOrder {
		selectableSpellings = append(selectableSpellings, string(selectableInterval))
	}

	return AggregationIntervalDomain{}, fmt.Errorf(
		"%w: 彙總刻度只能是 %s 其中之一",
		ErrKCandleValidation, strings.Join(selectableSpellings, "、"))
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
