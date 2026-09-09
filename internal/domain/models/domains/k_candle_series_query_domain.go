package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// KCandleSeriesQueryDomain holds one aggregated query and guarantees its own
// invariants. It is a plain time-range query plus a coarseness, so the rules about
// naming a trading symbol and not ending before it starts are answered once, by the
// range query it contains, rather than written down a second time here.
//
// The rules that are its own:
//
//   - **How long one candle covers is asked in one of two ways, never both.** A caller
//     either names a coarseness or says how many candles it can display and lets the
//     market decide. Both at once is refused: they contradict each other as easily as
//     they agree, and honouring either would let the other fail silently.
//   - **The range must not be cut into more buckets than one query may answer with.**
//     Decided from the range and the coarseness alone — before a single candle is read
//     — so an over-large ask costs nothing to refuse.
//
// **Buckets are counted over trading time, not over the clock.** A day of a market
// that shuts holds four and a half hours of candles, not twenty-four, and counting the
// closed hours is what used to make the system refuse a chart it had just chosen the
// coarseness for. One way of counting, used by the ceiling and the choosing alike.
type KCandleSeriesQueryDomain struct {
	rangeQuery  KCandleQueryDomain
	interval    AggregationIntervalDomain
	bucketCount int
}

// NewKCandleSeriesQueryDomain validates the query against every rule that applies to
// it, settling how long one candle covers on the way. The over-large refusal names
// both ways out, because narrowing the range and coarsening the interval are equally
// good answers and only the caller knows which it wanted.
//
// The market is handed in rather than looked up: how long a venue trades is a fact
// about the venue, and this object is not the place that knows which venue a symbol
// belongs to.
func NewKCandleSeriesQueryDomain(
	seriesQueryDto dto.KCandleSeriesQueryDto, marketDomain MarketDomain, maxBucketCount int,
) (KCandleSeriesQueryDomain, error) {
	rangeQuery, rangeValidationError := NewKCandleQueryDomain(seriesQueryDto.ToQueryDto())
	if rangeValidationError != nil {
		return KCandleSeriesQueryDomain{}, rangeValidationError
	}

	tradingTime := marketDomain.TradingTimeBetween(rangeQuery.StartTime(), rangeQuery.EndTime())

	interval, intervalError := intervalFor(seriesQueryDto, tradingTime, maxBucketCount)
	if intervalError != nil {
		return KCandleSeriesQueryDomain{}, intervalError
	}

	bucketCount := interval.SlotCount(tradingTime)
	if bucketCount > maxBucketCount {
		return KCandleSeriesQueryDomain{}, fmt.Errorf(
			"%w: 時間區間過大，請縮小區間；若指定了彙總刻度，也可以改用更長的一種（單次最多 %d 根）",
			ErrKCandleValidation, maxBucketCount)
	}

	return KCandleSeriesQueryDomain{
		rangeQuery:  rangeQuery,
		interval:    interval,
		bucketCount: bucketCount,
	}, nil
}

// intervalFor settles how long one candle covers, from whichever of the three things
// the caller said — a coarseness, a number of places to put candles in, or nothing at
// all — and refuses an ask that names the first two at once.
//
// Naming nothing is the caller that only knows which stretch its user is looking at.
// It gets a coarseness chosen the same way as one that named a display budget, against
// the only budget there is left: what one query may answer with. Before, it got one
// minute, which meant a stretch of any real length was answered by refusing it — the
// system turning down a chart nobody had asked to be dense.
//
// The choosing is capped by what one query may answer with as well as by what the
// caller can display, so that a coarseness this system picked can never come back
// refused by this system's own ceiling. Asking for more places than the ceiling allows
// is not an error: it means the ceiling is the tighter of the two, which is exactly
// what taking the smaller of them says.
func intervalFor(
	seriesQueryDto dto.KCandleSeriesQueryDto, tradingTime time.Duration, maxBucketCount int,
) (AggregationIntervalDomain, error) {
	if seriesQueryDto.DisplayableCandleCount != nil && seriesQueryDto.Interval != "" {
		return AggregationIntervalDomain{}, fmt.Errorf(
			"%w: 彙總刻度與可顯示根數只能挑一種說法", ErrKCandleValidation)
	}

	if seriesQueryDto.Interval != "" {
		interval, intervalValidationError := NewAggregationIntervalDomain(seriesQueryDto.Interval)
		if intervalValidationError != nil {
			return AggregationIntervalDomain{}, fmt.Errorf(
				"%w: %w", ErrKCandleValidation, intervalValidationError)
		}

		return interval, nil
	}

	if seriesQueryDto.DisplayableCandleCount != nil {
		displayableCandleCount := *seriesQueryDto.DisplayableCandleCount
		if displayableCandleCount <= 0 {
			return AggregationIntervalDomain{}, fmt.Errorf(
				"%w: 可顯示根數必須大於零", ErrKCandleValidation)
		}

		return NewFittingAggregationIntervalDomain(
			tradingTime, min(displayableCandleCount, maxBucketCount)), nil
	}

	// Saying neither is a caller with no opinion about how coarse a candle is, and the
	// only budget left to choose against is what one query may answer with at all.
	//
	// It is deliberately not written as "the same as asking for the ceiling's worth of
	// places". Those two agree on today's numbers and mean different things: a caller
	// that named a number said something about **its own display**, and one that named
	// nothing said only that it wants an answer. The day the ceiling moves, this branch
	// should follow it and the one above should not — folded together, that change would
	// silently redefine one of them.
	return NewFittingAggregationIntervalDomain(tradingTime, maxBucketCount), nil
}

// RangeQuery is the plain time-range query to read the source candles with.
func (kCandleSeriesQueryDomain KCandleSeriesQueryDomain) RangeQuery() KCandleQueryDomain {
	return kCandleSeriesQueryDomain.rangeQuery
}

// SeriesOf is the series those source candles make under this query. Asking the query
// for it keeps what a series is made of — which symbol, which interval — in one place;
// the caller only has to read the candles and hand them back.
func (kCandleSeriesQueryDomain KCandleSeriesQueryDomain) SeriesOf(
	kCandles []entities.KCandle,
) KCandleSeriesDomain {
	return NewKCandleSeriesDomain(
		kCandleSeriesQueryDomain.rangeQuery.Symbol(), kCandleSeriesQueryDomain.interval, kCandles)
}

// SourceCandleLimit is the most source candles this query's buckets can hold, plus
// one bucket's worth of spare. It is the right limit to read with: it can never cut
// the answer short, and it stops an over-wide read before it starts.
//
// The spare is what makes "never cut short" true. A read hands back the *earliest*
// candles up to its limit, so a limit that is even one candle tight loses the newest
// bar — the end of the chart nobody can afford to lose, and the end nobody looks at
// twice. The buckets are counted over trading time while the range's own two ends fall
// wherever the user dragged them, so the stretch can hold a little more than the
// buckets do; one spare bucket covers it.
func (kCandleSeriesQueryDomain KCandleSeriesQueryDomain) SourceCandleLimit() int {
	return kCandleSeriesQueryDomain.interval.SourceCandleCount(
		kCandleSeriesQueryDomain.bucketCount + 1)
}
