package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// KCandleContractSummaryDayDomain is KCandleSummaryDayDomain for contracts, merging through the contract series so missing index and premium lines stay missing.
type KCandleContractSummaryDayDomain struct {
	symbol   string
	day      time.Time
	revision int64
	kCandles []entities.KCandleContract
}

func NewKCandleContractSummaryDayDomain(
	symbol string, day time.Time, revision int64, kCandles []entities.KCandleContract,
) KCandleContractSummaryDayDomain {
	return KCandleContractSummaryDayDomain{symbol: symbol, day: day.UTC(), revision: revision, kCandles: kCandles}
}

func (summaryDayDomain KCandleContractSummaryDayDomain) ToEntity() entities.KCandleContractSummaryDay {
	summaries := make([]entities.KCandleContractSummary, 0, 25)
	for _, interval := range summaryIntervals() {
		for _, merged := range NewKCandleContractSeriesDomain(summaryDayDomain.symbol, interval, summaryDayDomain.kCandles).ToDto().KCandles {
			summaries = append(summaries, entities.KCandleContractSummary{
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
				TradeCount:          merged.TradeCount,
				MarkOpen:            merged.MarkOpen,
				MarkHigh:            merged.MarkHigh,
				MarkLow:             merged.MarkLow,
				MarkClose:           merged.MarkClose,
				IndexOpen:           merged.IndexOpen,
				IndexHigh:           merged.IndexHigh,
				IndexLow:            merged.IndexLow,
				IndexClose:          merged.IndexClose,
				PremiumIndexOpen:    merged.PremiumIndexOpen,
				PremiumIndexHigh:    merged.PremiumIndexHigh,
				PremiumIndexLow:     merged.PremiumIndexLow,
				PremiumIndexClose:   merged.PremiumIndexClose,
			})
		}
	}

	return entities.KCandleContractSummaryDay{
		Symbol:     summaryDayDomain.symbol,
		Day:        summaryDayDomain.day,
		Revision:   summaryDayDomain.revision,
		Summarized: true,
		Summaries:  summaries,
	}
}
