package domains

import (
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// KCandleSeriesDomain groups K candles into interval buckets, earliest first; empty buckets are omitted rather than invented, since no trade is not a flat trade.
type KCandleSeriesDomain struct {
	symbol   string
	interval AggregationIntervalDomain
	kCandles []entities.KCandle
}

func NewKCandleSeriesDomain(
	symbol string, interval AggregationIntervalDomain, kCandles []entities.KCandle,
) KCandleSeriesDomain {
	return KCandleSeriesDomain{symbol: symbol, interval: interval, kCandles: kCandles}
}

// Buckets is the single place candles become a grid, shared by queries and indicator calculations so they never disagree on bucket starts.
func (kCandleSeriesDomain KCandleSeriesDomain) Buckets() []KCandleBucketDomain {
	kCandlesByBucketStart := make(map[time.Time][]entities.KCandle)
	bucketStarts := make([]time.Time, 0, len(kCandleSeriesDomain.kCandles))

	for _, kCandle := range kCandleSeriesDomain.kCandles {
		bucketStart := kCandleSeriesDomain.interval.BucketStart(kCandle.OpenTime)
		if _, alreadyOpened := kCandlesByBucketStart[bucketStart]; !alreadyOpened {
			bucketStarts = append(bucketStarts, bucketStart)
		}

		kCandlesByBucketStart[bucketStart] = append(kCandlesByBucketStart[bucketStart], kCandle)
	}

	slices.SortFunc(bucketStarts, func(former time.Time, latter time.Time) int {
		return former.Compare(latter)
	})

	buckets := make([]KCandleBucketDomain, 0, len(bucketStarts))
	for _, bucketStart := range bucketStarts {
		buckets = append(
			buckets, NewKCandleBucketDomain(bucketStart, kCandlesByBucketStart[bucketStart]))
	}

	return buckets
}

// ToDto returns the merged series earliest first; no candles is a legitimate empty series.
func (kCandleSeriesDomain KCandleSeriesDomain) ToDto() dto.KCandleSeriesDto {
	buckets := kCandleSeriesDomain.Buckets()

	aggregatedKCandles := make([]dto.KCandleDto, 0, len(buckets))
	for _, bucket := range buckets {
		aggregatedKCandles = append(aggregatedKCandles, bucket.ToDto())
	}

	return dto.KCandleSeriesDto{
		Symbol:   kCandleSeriesDomain.symbol,
		Interval: string(kCandleSeriesDomain.interval.Value()),
		KCandles: aggregatedKCandles,
	}
}
