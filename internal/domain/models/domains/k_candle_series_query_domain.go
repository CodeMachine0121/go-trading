package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// KCandleSeriesQueryDomain is a range query plus a coarseness, given either as an interval or a displayable count but never both, with the bucket count capped per query.
// Buckets are counted by walking market sessions rather than dividing a duration, which would undercount for markets that close.
type KCandleSeriesQueryDomain struct {
	rangeQuery  KCandleQueryDomain
	interval    AggregationIntervalDomain
	bucketCount int
}

// NewKCandleSeriesQueryDomain validates and settles the interval; the over-large refusal names both narrowing the range and coarsening the interval.
func NewKCandleSeriesQueryDomain(
	seriesQueryDto dto.KCandleSeriesQueryDto, marketDomain MarketDomain, maxBucketCount int,
) (KCandleSeriesQueryDomain, error) {
	rangeQuery, rangeValidationError := NewKCandleQueryDomain(seriesQueryDto.ToQueryDto())
	if rangeValidationError != nil {
		return KCandleSeriesQueryDomain{}, rangeValidationError
	}

	interval, intervalError := intervalFor(
		seriesQueryDto, marketDomain, rangeQuery.StartTime(), rangeQuery.EndTime(), maxBucketCount)
	if intervalError != nil {
		return KCandleSeriesQueryDomain{}, intervalError
	}

	bucketCount := interval.TradingSlotCount(
		marketDomain, rangeQuery.StartTime(), rangeQuery.EndTime())
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

// intervalFor settles the interval from an explicit interval, a displayable count, or nothing, refusing the first two together.
// A chosen interval is capped by the per-query ceiling so the system never picks one its own ceiling would refuse.
func intervalFor(
	seriesQueryDto dto.KCandleSeriesQueryDto,
	marketDomain MarketDomain,
	startTime time.Time,
	endTime time.Time,
	maxBucketCount int,
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
			marketDomain, startTime, endTime,
			min(displayableCandleCount, maxBucketCount)), nil
	}

	// Kept separate from the displayable-count branch on purpose: if the ceiling changes, this branch should follow it and that one should not.
	return NewFittingAggregationIntervalDomain(
		marketDomain, startTime, endTime, maxBucketCount), nil
}

func (kCandleSeriesQueryDomain KCandleSeriesQueryDomain) RangeQuery() KCandleQueryDomain {
	return kCandleSeriesQueryDomain.rangeQuery
}

// SeriesOf keeps the series' symbol and interval with the query; callers only supply the candles.
func (kCandleSeriesQueryDomain KCandleSeriesQueryDomain) SeriesOf(
	kCandles []entities.KCandle,
) KCandleSeriesDomain {
	return NewKCandleSeriesDomain(
		kCandleSeriesQueryDomain.rangeQuery.Symbol(), kCandleSeriesQueryDomain.interval, kCandles)
}

// ContractSeriesOf cuts the same buckets as SeriesOf, merged the contract way.
func (kCandleSeriesQueryDomain KCandleSeriesQueryDomain) ContractSeriesOf(
	kCandleContracts []entities.KCandleContract,
) KCandleContractSeriesDomain {
	return NewKCandleContractSeriesDomain(
		kCandleSeriesQueryDomain.rangeQuery.Symbol(), kCandleSeriesQueryDomain.interval, kCandleContracts)
}

// SourceCandleLimit is the source-candle capacity of the buckets plus one spare bucket, because a read returns the earliest candles and a tight limit would drop the newest bar.
func (kCandleSeriesQueryDomain KCandleSeriesQueryDomain) SourceCandleLimit() int {
	return kCandleSeriesQueryDomain.interval.SourceCandleCount(
		kCandleSeriesQueryDomain.bucketCount + 1)
}
