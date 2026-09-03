package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleBucketDomain is the K candles that fell into one bucket of the aggregation
// grid, and the single candle they make together. It is the only place the merging
// rule is written down: the bucket opens where its earliest candle opened and closes
// where its latest candle closed, reaches as high and as low as any candle in it, and
// traded everything they all traded.
//
// The candles need not arrive in time order — earliest and latest are decided by open
// time, not by position.
type KCandleBucketDomain struct {
	bucketStart time.Time
	kCandles    []entities.KCandle
}

func NewKCandleBucketDomain(bucketStart time.Time, kCandles []entities.KCandle) KCandleBucketDomain {
	return KCandleBucketDomain{bucketStart: bucketStart, kCandles: kCandles}
}

// ToDto merges the bucket into the one candle that stands for it. The open time it
// carries is the bucket's own start, not any candle's — that is what makes two
// queries over the same stretch of market line up.
func (kCandleBucketDomain KCandleBucketDomain) ToDto() dto.KCandleDto {
	mergedKCandle := dto.KCandleDto{OpenTime: kCandleBucketDomain.bucketStart}

	earliestKCandle := entities.KCandle{}
	latestKCandle := entities.KCandle{}

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
		mergedKCandle.QuoteVolume = mergedKCandle.QuoteVolume.Add(kCandle.QuoteVolume)
		mergedKCandle.TakerBuyBaseVolume = mergedKCandle.TakerBuyBaseVolume.Add(kCandle.TakerBuyBaseVolume)
		mergedKCandle.TakerBuyQuoteVolume = mergedKCandle.TakerBuyQuoteVolume.Add(kCandle.TakerBuyQuoteVolume)
	}

	mergedKCandle.Symbol = earliestKCandle.Symbol
	mergedKCandle.Open = earliestKCandle.Open
	mergedKCandle.Close = latestKCandle.Close

	return mergedKCandle
}

// ToVo merges the bucket into the one candle an indicator script sees. It is the
// same merge — it asks ToDto for it rather than repeating the rule — turned into the
// shape a script is handed: plain numbers, and the open time as seconds so that a
// script can never reach the clock through it.
//
// A script therefore cannot tell an aggregated candle from a stored one, which is
// what lets one algorithm run at any coarseness without knowing it has.
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
		QuoteVolume:         mergedKCandle.QuoteVolume.InexactFloat64(),
		TakerBuyBaseVolume:  mergedKCandle.TakerBuyBaseVolume.InexactFloat64(),
		TakerBuyQuoteVolume: mergedKCandle.TakerBuyQuoteVolume.InexactFloat64(),
	}
}
