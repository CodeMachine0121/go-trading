package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleBucketDomain is the single place candles in one grid bucket are merged; earliest and latest are decided by open time, so input order does not matter.
type KCandleBucketDomain struct {
	bucketStart time.Time
	kCandles    []entities.KCandle
}

func NewKCandleBucketDomain(bucketStart time.Time, kCandles []entities.KCandle) KCandleBucketDomain {
	return KCandleBucketDomain{bucketStart: bucketStart, kCandles: kCandles}
}

// OpenTime is the bucket's own start rather than any candle's, so readings of the same stretch line up.
func (kCandleBucketDomain KCandleBucketDomain) OpenTime() time.Time {
	return kCandleBucketDomain.bucketStart
}

// ToDto merges the bucket into one candle stamped with the bucket's start.
func (kCandleBucketDomain KCandleBucketDomain) ToDto() dto.KCandleDto {
	mergedKCandle := dto.KCandleDto{OpenTime: kCandleBucketDomain.bucketStart}

	earliestKCandle := entities.KCandle{}
	latestKCandle := entities.KCandle{}
	// Optional figures stay absent when no candle reports them, rather than becoming a confident zero.
	quoteVolume := OptionalFigureDomain{}
	takerBuyBaseVolume := OptionalFigureDomain{}
	takerBuyQuoteVolume := OptionalFigureDomain{}

	for index, kCandle := range kCandleBucketDomain.kCandles {
		if index == 0 || kCandle.OpenTime.Before(earliestKCandle.OpenTime) {
			earliestKCandle = kCandle
		}
		if index == 0 || kCandle.OpenTime.After(latestKCandle.OpenTime) {
			latestKCandle = kCandle
		}
		if index == 0 || kCandle.High.GreaterThan(mergedKCandle.High) {
			mergedKCandle.High = kCandle.High
		}
		if index == 0 || kCandle.Low.LessThan(mergedKCandle.Low) {
			mergedKCandle.Low = kCandle.Low
		}

		mergedKCandle.Volume = mergedKCandle.Volume.Add(kCandle.Volume)
		quoteVolume = quoteVolume.Plus(kCandle.QuoteVolume)
		takerBuyBaseVolume = takerBuyBaseVolume.Plus(kCandle.TakerBuyBaseVolume)
		takerBuyQuoteVolume = takerBuyQuoteVolume.Plus(kCandle.TakerBuyQuoteVolume)
	}

	mergedKCandle.Symbol = earliestKCandle.Symbol
	mergedKCandle.Open = earliestKCandle.Open
	mergedKCandle.Close = latestKCandle.Close
	mergedKCandle.QuoteVolume = quoteVolume.Value()
	mergedKCandle.TakerBuyBaseVolume = takerBuyBaseVolume.Value()
	mergedKCandle.TakerBuyQuoteVolume = takerBuyQuoteVolume.Value()

	return mergedKCandle
}

// ToVo is ToDto's merge in script shape, with the open time as Unix seconds so scripts cannot reach the clock and cannot tell aggregated candles from stored ones.
func (kCandleBucketDomain KCandleBucketDomain) ToVo() vo.KCandleVo {
	mergedKCandle := kCandleBucketDomain.ToDto()

	return vo.KCandleVo{
		Symbol:              mergedKCandle.Symbol,
		OpenTimeUnixSeconds: mergedKCandle.OpenTime.UTC().Unix(),
		Open:                mergedKCandle.Open.InexactFloat64(),
		High:                mergedKCandle.High.InexactFloat64(),
		Low:                 mergedKCandle.Low.InexactFloat64(),
		Close:               mergedKCandle.Close.InexactFloat64(),
		Volume:              mergedKCandle.Volume.InexactFloat64(),
		QuoteVolume:         NewOptionalFigureDomain(mergedKCandle.QuoteVolume).AsScriptFigure(),
		TakerBuyBaseVolume:  NewOptionalFigureDomain(mergedKCandle.TakerBuyBaseVolume).AsScriptFigure(),
		TakerBuyQuoteVolume: NewOptionalFigureDomain(mergedKCandle.TakerBuyQuoteVolume).AsScriptFigure(),
	}
}
