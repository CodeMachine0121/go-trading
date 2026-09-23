package domains

import (
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// KCandleContractSeriesDomain is a stretch of contract K candles merged into one
// candle per interval bucket — the contract counterpart of KCandleSeriesDomain.
//
// The traded figures merge exactly as spot ones do. The three lines the venue computes
// beside them merge the same way each on its own: first open, last close, highest
// high, lowest low. **A bucket holding even one candle without its index price or
// premium index has no such line at all** — the rest could be merged into a figure
// that looks whole and quietly leaves out minutes, which is worse than saying nothing.
type KCandleContractSeriesDomain struct {
	symbol   string
	interval AggregationIntervalDomain
	kCandles []entities.KCandleContract
}

func NewKCandleContractSeriesDomain(
	symbol string, interval AggregationIntervalDomain, kCandles []entities.KCandleContract,
) KCandleContractSeriesDomain {
	return KCandleContractSeriesDomain{symbol: symbol, interval: interval, kCandles: kCandles}
}

// ToDto merges each bucket and hands the series outwards, earliest bucket first. A
// bucket nothing was held for produces nothing: no gap is filled.
func (seriesDomain KCandleContractSeriesDomain) ToDto() dto.KCandleContractSeriesDto {
	kCandlesByBucketStart := make(map[time.Time][]entities.KCandleContract)
	bucketStarts := make([]time.Time, 0, len(seriesDomain.kCandles))
	for _, kCandle := range seriesDomain.kCandles {
		bucketStart := seriesDomain.interval.BucketStart(kCandle.OpenTime)
		if _, alreadyOpened := kCandlesByBucketStart[bucketStart]; !alreadyOpened {
			bucketStarts = append(bucketStarts, bucketStart)
		}
		kCandlesByBucketStart[bucketStart] = append(kCandlesByBucketStart[bucketStart], kCandle)
	}
	slices.SortFunc(bucketStarts, func(former time.Time, latter time.Time) int {
		return former.Compare(latter)
	})

	mergedKCandles := make([]dto.KCandleContractDto, 0, len(bucketStarts))
	for _, bucketStart := range bucketStarts {
		mergedKCandles = append(mergedKCandles,
			seriesDomain.mergeBucket(bucketStart, kCandlesByBucketStart[bucketStart]))
	}

	return dto.KCandleContractSeriesDto{
		Symbol:   seriesDomain.symbol,
		Interval: string(seriesDomain.interval.Value()),
		KCandles: mergedKCandles,
	}
}

// mergeBucket merges one bucket's candles into one.
func (seriesDomain KCandleContractSeriesDomain) mergeBucket(
	bucketStart time.Time, kCandles []entities.KCandleContract,
) dto.KCandleContractDto {
	slices.SortFunc(kCandles, func(former entities.KCandleContract, latter entities.KCandleContract) int {
		return former.OpenTime.Compare(latter.OpenTime)
	})
	earliest, latest := kCandles[0], kCandles[len(kCandles)-1]

	merged := dto.KCandleContractDto{
		Symbol:   earliest.Symbol,
		OpenTime: bucketStart.UTC(),
		Open:     earliest.Open, High: earliest.High, Low: earliest.Low, Close: latest.Close,
		MarkOpen: earliest.MarkOpen, MarkHigh: earliest.MarkHigh, MarkLow: earliest.MarkLow,
		MarkClose: latest.MarkClose,
	}
	indexLine := newMergedPriceLine()
	premiumIndexLine := newMergedPriceLine()

	for _, kCandle := range kCandles {
		merged.High = decimal.Max(merged.High, kCandle.High)
		merged.Low = decimal.Min(merged.Low, kCandle.Low)
		merged.Volume = merged.Volume.Add(kCandle.Volume)
		merged.QuoteVolume = merged.QuoteVolume.Add(kCandle.QuoteVolume)
		merged.TakerBuyBaseVolume = merged.TakerBuyBaseVolume.Add(kCandle.TakerBuyBaseVolume)
		merged.TakerBuyQuoteVolume = merged.TakerBuyQuoteVolume.Add(kCandle.TakerBuyQuoteVolume)
		merged.TradeCount += kCandle.TradeCount
		merged.MarkHigh = decimal.Max(merged.MarkHigh, kCandle.MarkHigh)
		merged.MarkLow = decimal.Min(merged.MarkLow, kCandle.MarkLow)
		indexLine = indexLine.including(kCandle.IndexOpen, kCandle.IndexHigh, kCandle.IndexLow, kCandle.IndexClose)
		premiumIndexLine = premiumIndexLine.including(
			kCandle.PremiumIndexOpen, kCandle.PremiumIndexHigh, kCandle.PremiumIndexLow, kCandle.PremiumIndexClose)
	}

	merged.IndexOpen, merged.IndexHigh, merged.IndexLow, merged.IndexClose = indexLine.figures()
	merged.PremiumIndexOpen, merged.PremiumIndexHigh, merged.PremiumIndexLow, merged.PremiumIndexClose =
		premiumIndexLine.figures()

	return merged
}

// mergedPriceLine is one optional price line being merged across a bucket's candles,
// taken in open-time order. It stays whole only while every candle had the line.
type mergedPriceLine struct {
	isStarted bool
	isBroken  bool
	open      decimal.Decimal
	high      decimal.Decimal
	low       decimal.Decimal
	close     decimal.Decimal
}

func newMergedPriceLine() mergedPriceLine {
	return mergedPriceLine{}
}

// including folds the next candle's figures of the line in. A candle without the line
// breaks it for the whole bucket.
func (line mergedPriceLine) including(open, high, low, close decimal.NullDecimal) mergedPriceLine {
	if line.isBroken || !open.Valid || !high.Valid || !low.Valid || !close.Valid {
		line.isBroken = true

		return line
	}

	if !line.isStarted {
		return mergedPriceLine{isStarted: true, open: open.Decimal, high: high.Decimal,
			low: low.Decimal, close: close.Decimal}
	}

	line.high = decimal.Max(line.high, high.Decimal)
	line.low = decimal.Min(line.low, low.Decimal)
	line.close = close.Decimal

	return line
}

// figures hands the merged line out, or four absent figures when a candle lacked it.
func (line mergedPriceLine) figures() (decimal.NullDecimal, decimal.NullDecimal, decimal.NullDecimal, decimal.NullDecimal) {
	if line.isBroken || !line.isStarted {
		return decimal.NullDecimal{}, decimal.NullDecimal{}, decimal.NullDecimal{}, decimal.NullDecimal{}
	}

	return decimal.NewNullDecimal(line.open), decimal.NewNullDecimal(line.high),
		decimal.NewNullDecimal(line.low), decimal.NewNullDecimal(line.close)
}
