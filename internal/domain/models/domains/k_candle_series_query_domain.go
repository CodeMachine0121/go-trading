package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// KCandleSeriesQueryDomain holds one aggregated query and guarantees its own
// invariants. It is a plain time-range query plus an interval, so the rules about
// naming a trading symbol and not ending before it starts are answered once, by the
// range query it contains, rather than written down a second time here.
//
// The rule that is its own: the range must not be cut into more buckets than one
// query may answer with. That is decided from the range and the interval alone —
// before a single candle is read — so an over-large ask costs nothing to refuse.
type KCandleSeriesQueryDomain struct {
	rangeQuery  KCandleQueryDomain
	interval    AggregationIntervalDomain
	bucketCount int
}

// NewKCandleSeriesQueryDomain validates the query against every rule that applies to
// it, refusing an ask whose range holds more buckets than maxBucketCount. The refusal
// names both ways out, because narrowing the range and coarsening the interval are
// equally good answers and only the caller knows which it wanted.
func NewKCandleSeriesQueryDomain(
	seriesQueryDto dto.KCandleSeriesQueryDto, maxBucketCount int,
) (KCandleSeriesQueryDomain, error) {
	rangeQuery, rangeValidationError := NewKCandleQueryDomain(seriesQueryDto.ToQueryDto())
	if rangeValidationError != nil {
		return KCandleSeriesQueryDomain{}, rangeValidationError
	}

	interval, intervalValidationError := NewAggregationIntervalDomain(seriesQueryDto.Interval)
	if intervalValidationError != nil {
		return KCandleSeriesQueryDomain{}, fmt.Errorf(
			"%w: %w", ErrKCandleValidation, intervalValidationError)
	}

	bucketCount := interval.BucketCount(rangeQuery.StartTime(), rangeQuery.EndTime())
	if bucketCount > maxBucketCount {
		return KCandleSeriesQueryDomain{}, fmt.Errorf(
			"%w: 時間區間過大，請縮小區間或改用更長的彙總刻度（單次最多 %d 根）",
			ErrKCandleValidation, maxBucketCount)
	}

	return KCandleSeriesQueryDomain{
		rangeQuery:  rangeQuery,
		interval:    interval,
		bucketCount: bucketCount,
	}, nil
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

// SourceCandleLimit is the most source candles this query's buckets can possibly
// hold, which is the right limit to read with: it can never cut the answer short,
// and it stops an over-wide read before it starts.
func (kCandleSeriesQueryDomain KCandleSeriesQueryDomain) SourceCandleLimit() int {
	return kCandleSeriesQueryDomain.interval.SourceCandleCount(kCandleSeriesQueryDomain.bucketCount)
}
