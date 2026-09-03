package domains

import (
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// KCandleSeriesDomain is the K candles read for one aggregated query, together with
// the interval they are to be read at. It knows how a pile of candles becomes a
// series: each candle joins the bucket its open time falls into, each bucket becomes
// one candle, and the buckets run earliest first.
//
// A bucket that no candle fell into is not in the series at all. Nothing is filled in
// for it — an invented candle reads exactly like a real one, and a market that did not
// trade is not the same as a market that traded flat.
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

// ToDto groups the candles into their buckets and hands back the merged series,
// earliest first. Reading no candles is a legitimate answer: an empty series.
func (kCandleSeriesDomain KCandleSeriesDomain) ToDto() dto.KCandleSeriesDto {
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

	aggregatedKCandles := make([]dto.KCandleDto, 0, len(bucketStarts))
	for _, bucketStart := range bucketStarts {
		aggregatedKCandles = append(
			aggregatedKCandles,
			NewKCandleBucketDomain(bucketStart, kCandlesByBucketStart[bucketStart]).ToDto())
	}

	return dto.KCandleSeriesDto{
		Symbol:   kCandleSeriesDomain.symbol,
		Interval: string(kCandleSeriesDomain.interval.Value()),
		KCandles: aggregatedKCandles,
	}
}
