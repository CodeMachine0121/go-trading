package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// KCandleSummaryDayDomain merges one UTC day of spot minutes into its hourly and daily summaries through the series merge itself, so a summary can never disagree with merging the minutes live.
type KCandleSummaryDayDomain struct {
	symbol   string
	day      time.Time
	revision int64
	kCandles []entities.KCandle
}

func NewKCandleSummaryDayDomain(
	symbol string, day time.Time, revision int64, kCandles []entities.KCandle,
) KCandleSummaryDayDomain {
	return KCandleSummaryDayDomain{symbol: symbol, day: day.UTC(), revision: revision, kCandles: kCandles}
}

// ToEntity is marked summarized even with no minutes, so a day without trading is not merged again on every request.
func (summaryDayDomain KCandleSummaryDayDomain) ToEntity() entities.KCandleSummaryDay {
	summaries := make([]entities.KCandleSummary, 0, 25)
	for _, interval := range summaryIntervals() {
		for _, bucket := range NewKCandleSeriesDomain(summaryDayDomain.symbol, interval, summaryDayDomain.kCandles).Buckets() {
			merged := bucket.ToDto()
			summaries = append(summaries, entities.KCandleSummary{
				Symbol:              summaryDayDomain.symbol,
				Interval:            string(interval.value),
				OpenTime:            merged.OpenTime,
				Open:                merged.Open,
				High:                merged.High,
				Low:                 merged.Low,
				Close:               merged.Close,
				Volume:              merged.Volume,
				QuoteVolume:         merged.QuoteVolume,
				TakerBuyBaseVolume:  merged.TakerBuyBaseVolume,
				TakerBuyQuoteVolume: merged.TakerBuyQuoteVolume,
			})
		}
	}

	return entities.KCandleSummaryDay{
		Symbol:     summaryDayDomain.symbol,
		Day:        summaryDayDomain.day,
		Revision:   summaryDayDomain.revision,
		Summarized: true,
		Summaries:  summaries,
	}
}
