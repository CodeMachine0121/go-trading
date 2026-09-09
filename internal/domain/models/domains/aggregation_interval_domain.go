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
// **Backfill leans on that invariant.** It rounds where it starts fetching down to the
// coarsest interval's bucket edge, and that is enough for every interval only because
// each length divides a day: midnight is a whole multiple of all of them. A length
// that broke the invariant would not merely drift — it would leave the oldest bucket
// of that coarseness beginning part way through itself again. Read
// NewCoarsestAggregationIntervalDomain before adding a row.
//
// Supporting one more interval means adding a row here and a constant in vo — nothing
// downstream branches per interval.
var selectableAggregationIntervals = []selectableAggregationInterval{
	{value: vo.AggregationIntervalOneMinute, duration: KCandleInterval},
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
//
// The refusal carries the reason alone, with no sentinel of its own. Which kind of
// validation an unrecognised interval counts as is the caller's question, not the
// interval's: the same bad spelling is a K candle query problem in one place and a
// strategy problem in another. Each caller wraps this reason in its own sentinel.
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

// NewCoarsestAggregationIntervalDomain is the longest interval on offer, and it is
// what anything aligning to a bucket edge should ask for rather than naming a length
// of its own.
//
// Aligning a moment to this one's edge aligns it for every interval at once, because
// each declarable length divides a day and this one *is* a day: its edges are a subset
// of every other interval's. So a caller wanting "a moment that no bucket begins after"
// asks here, and keeps working the day a coarser interval is added.
//
// It cannot fail — the set is never empty — so unlike NewAggregationIntervalDomain
// there is nothing to declare and nothing to refuse.
//
// It reads the lengths rather than trusting the set's shortest-first order. That order
// is a comment, and nothing enforces it: a row inserted in the wrong place would
// silently hand back an interval that is not the coarsest, and every bucket edge
// derived from it would be wrong while looking ordinary.
func NewCoarsestAggregationIntervalDomain() AggregationIntervalDomain {
	return newAggregationIntervalDomain(coarsestSelectableAggregationInterval())
}

// NewFittingAggregationIntervalDomain is the finest coarseness that fits: given a
// stretch of a market's time and how many candles the caller can display, the shortest
// interval whose slots do not outnumber the places to put them.
//
// It cannot fail. A caller looking at more market than even the coarsest interval can
// fit into its display gets that coarsest one, because seeing the stretch too densely
// packed beats seeing nothing at all — and refusing would leave a chart with no answer
// at the one moment the user zoomed furthest out. What stops that from handing back
// more candles than one query may answer with is the caller's own ceiling check, which
// counts the same way this does.
//
// **It asks the market how many buckets a stretch holds, per candidate.** It used to
// take a duration and divide, which is the assumption this whole line of changes has
// been unpicking: a market that shuts does not offer its clock evenly, and a bucket
// that catches a minute of trading is a whole candle. Counting is a walk over the
// market's sessions, so the market has to be here.
//
// It walks the set finest-first by reading the lengths rather than trusting the set's
// order, for the same reason the coarsest one does: that order is a comment, and a row
// inserted in the wrong place would silently hand back an interval that is not the
// finest fitting one.
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

// coarsestSelectableAggregationInterval is the longest interval on offer, which is
// both what "no coarseness declared at all" falls back to and what a stretch too wide
// for any of them settles on.
func coarsestSelectableAggregationInterval() selectableAggregationInterval {
	coarsestInterval := selectableAggregationIntervals[0]
	for _, selectableInterval := range selectableAggregationIntervals {
		if selectableInterval.duration > coarsestInterval.duration {
			coarsestInterval = selectableInterval
		}
	}

	return coarsestInterval
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

// TradingSlotCount is how many of this coarseness a stretch holds — the number of
// values a caller looking at that much market is asking for.
//
// **It asks the market to count buckets rather than dividing a duration.** The
// division it replaced looked right and was not: a bucket is a slot with edges, not a
// quantity of time, so one that catches a single minute of trading is a whole candle.
// Dividing therefore undercounts wherever a bucket reaches past a session's edges, and
// worse the coarser it is — a Taiwan trading day is five hourly buckets, not four, and
// five Taiwan days are five daily buckets, not the fifth of one. The market is the only
// thing that can count them, because counting means walking its sessions.
//
// That is also why this takes the market and the two moments rather than a duration.
// The earlier split — the market says how long it trades, the interval says how many of
// itself fit — reads well and hides the bug in its second half: it assumes time can be
// divided into buckets, and it cannot.
//
// It stops at one. A stretch holding no trading at all reaches here, and the floor is
// load-bearing: an indicator calculation refuses such a window before asking, but a
// series query deliberately does not — looking at a Saturday is a thing a user can do,
// and the answer is an empty series rather than a refusal.
func (aggregationIntervalDomain AggregationIntervalDomain) TradingSlotCount(
	marketDomain MarketDomain, startTime time.Time, endTime time.Time,
) int {
	return max(1, marketDomain.TradingBucketCountBetween(
		startTime, endTime, aggregationIntervalDomain.duration))
}

// SourceCandleCount is the most stored K candles the given number of buckets can hold.
// One bucket holds as many candles as its length fits, and a trading symbol holds at
// most one candle per K candle slot, so this is an upper bound the data cannot
// exceed — which is exactly what a read limit needs to be.
func (aggregationIntervalDomain AggregationIntervalDomain) SourceCandleCount(bucketCount int) int {
	candlesPerBucket := int(aggregationIntervalDomain.duration / KCandleInterval)

	return bucketCount * candlesPerBucket
}
