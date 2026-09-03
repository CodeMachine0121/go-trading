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

// Buckets sorts the candles into the buckets their open times fall into and hands
// them back earliest first. A bucket no candle fell into is not among them at all.
//
// This is the one place a pile of candles becomes a grid, and it is public because
// it has two customers: a query wants each bucket as a candle to look at, an
// indicator calculation wants each bucket as a candle to compute from. Were they to
// group candles separately, the two would sooner or later disagree about where a
// bucket starts — and the symptom of that is a line quietly drawn one bucket out of
// place, with nothing reported.
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

// ToDto hands back the merged series, earliest first. Reading no candles is a
// legitimate answer: an empty series.
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
