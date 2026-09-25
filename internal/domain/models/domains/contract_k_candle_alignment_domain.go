package domains

import (
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ContractKCandleAlignmentDomain lines funding settlements and position statistics up with finished contract bars for a script.
// Every figure on a bar comes from strictly before its close (a record stamped at the close belongs to the next bar), and missing figures are zero, as ContractKCandleVo documents.
type ContractKCandleAlignmentDomain struct {
	interval AggregationIntervalDomain
	// buckets are oldest first and never empty.
	buckets []dto.KCandleContractDto
}

// Only buckets with a contract K candle become bars, whatever settlements or statistics the stretch holds.
func newContractKCandleAlignmentDomain(
	interval AggregationIntervalDomain, buckets []dto.KCandleContractDto,
) ContractKCandleAlignmentDomain {
	return ContractKCandleAlignmentDomain{interval: interval, buckets: buckets}
}

// SettlementLeadInCutoff is the first bar's open; the latest settlement before it (the rate in force at the start) must be queried separately since it may be arbitrarily far back.
func (alignmentDomain ContractKCandleAlignmentDomain) SettlementLeadInCutoff() time.Time {
	return alignmentDomain.buckets[0].OpenTime.UTC()
}

// SettlementQuery spans from the first bar's open to the last bar's close.
func (alignmentDomain ContractKCandleAlignmentDomain) SettlementQuery() KCandleQueryDomain {
	return alignmentDomain.queryReachingBack(0)
}

// SettlementReadLimit assumes no contract settles more often than hourly.
func (alignmentDomain ContractKCandleAlignmentDomain) SettlementReadLimit() int {
	return alignmentDomain.readLimitReachingBack(0, time.Hour)
}

// StatisticQuery reaches one statistic interval before the first bar, the furthest a finer bar may look back.
func (alignmentDomain ContractKCandleAlignmentDomain) StatisticQuery() KCandleQueryDomain {
	return alignmentDomain.queryReachingBack(ContractPositionStatisticInterval)
}

func (alignmentDomain ContractKCandleAlignmentDomain) StatisticReadLimit() int {
	return alignmentDomain.readLimitReachingBack(ContractPositionStatisticInterval, ContractPositionStatisticInterval)
}

// Aligning gives each bar the rate of the latest settlement before its close, whether one fell inside the bar, and the latest position statistic before its close if inside the bar or within one statistic interval of the close (else zeros, so stale data never looks current).
// Inputs may arrive in any order; the settlements should include the lead-in one from SettlementLeadInCutoff.
func (alignmentDomain ContractKCandleAlignmentDomain) Aligning(
	settlements []entities.ContractFundingRateSettlement,
	statistics []entities.ContractPositionStatistic,
) []vo.ContractKCandleVo {
	earliestFirstSettlements := slices.Clone(settlements)
	slices.SortFunc(earliestFirstSettlements, func(former, latter entities.ContractFundingRateSettlement) int {
		return former.SettlementTime.Compare(latter.SettlementTime)
	})
	earliestFirstStatistics := slices.Clone(statistics)
	slices.SortFunc(earliestFirstStatistics, func(former, latter entities.ContractPositionStatistic) int {
		return former.StatisticTime.Compare(latter.StatisticTime)
	})

	contractKCandleVos := make([]vo.ContractKCandleVo, 0, len(alignmentDomain.buckets))
	settlementsBeforeClose := 0
	statisticsBeforeClose := 0
	for _, bucket := range alignmentDomain.buckets {
		bucketStart := bucket.OpenTime.UTC()
		bucketEnd := bucketStart.Add(alignmentDomain.interval.duration)

		for settlementsBeforeClose < len(earliestFirstSettlements) &&
			earliestFirstSettlements[settlementsBeforeClose].SettlementTime.Before(bucketEnd) {
			settlementsBeforeClose++
		}
		for statisticsBeforeClose < len(earliestFirstStatistics) &&
			earliestFirstStatistics[statisticsBeforeClose].StatisticTime.Before(bucketEnd) {
			statisticsBeforeClose++
		}

		contractKCandleVo := vo.ContractKCandleVo{
			KCandleVo: vo.KCandleVo{
				Symbol:              bucket.Symbol,
				OpenTimeUnixSeconds: bucketStart.Unix(),
				Open:                bucket.Open.InexactFloat64(),
				High:                bucket.High.InexactFloat64(),
				Low:                 bucket.Low.InexactFloat64(),
				Close:               bucket.Close.InexactFloat64(),
				Volume:              bucket.Volume.InexactFloat64(),
				QuoteVolume:         bucket.QuoteVolume.InexactFloat64(),
				TakerBuyBaseVolume:  bucket.TakerBuyBaseVolume.InexactFloat64(),
				TakerBuyQuoteVolume: bucket.TakerBuyQuoteVolume.InexactFloat64(),
			},
			TradeCount: bucket.TradeCount,
			Mark: vo.PriceLineVo{
				Open:  bucket.MarkOpen.InexactFloat64(),
				High:  bucket.MarkHigh.InexactFloat64(),
				Low:   bucket.MarkLow.InexactFloat64(),
				Close: bucket.MarkClose.InexactFloat64(),
			},
			Index: vo.PriceLineVo{
				Open:  NewOptionalFigureDomain(bucket.IndexOpen).AsScriptFigure(),
				High:  NewOptionalFigureDomain(bucket.IndexHigh).AsScriptFigure(),
				Low:   NewOptionalFigureDomain(bucket.IndexLow).AsScriptFigure(),
				Close: NewOptionalFigureDomain(bucket.IndexClose).AsScriptFigure(),
			},
			PremiumIndex: vo.PriceLineVo{
				Open:  NewOptionalFigureDomain(bucket.PremiumIndexOpen).AsScriptFigure(),
				High:  NewOptionalFigureDomain(bucket.PremiumIndexHigh).AsScriptFigure(),
				Low:   NewOptionalFigureDomain(bucket.PremiumIndexLow).AsScriptFigure(),
				Close: NewOptionalFigureDomain(bucket.PremiumIndexClose).AsScriptFigure(),
			},
		}

		if settlementsBeforeClose > 0 {
			latestSettlement := earliestFirstSettlements[settlementsBeforeClose-1]
			contractKCandleVo.FundingRate = latestSettlement.FundingRate.InexactFloat64()
			contractKCandleVo.FundingSettledInBar = !latestSettlement.SettlementTime.Before(bucketStart)
		}

		earliestRecentEnough := bucketStart
		if statisticReach := bucketEnd.Add(-ContractPositionStatisticInterval); statisticReach.Before(bucketStart) {
			earliestRecentEnough = statisticReach
		}
		if statisticsBeforeClose > 0 {
			latestStatistic := earliestFirstStatistics[statisticsBeforeClose-1]
			if !latestStatistic.StatisticTime.Before(earliestRecentEnough) {
				contractKCandleVo.OpenInterest = latestStatistic.OpenInterest.InexactFloat64()
				contractKCandleVo.OpenInterestValue = latestStatistic.OpenInterestValue.InexactFloat64()
				contractKCandleVo.AccountLongShare = latestStatistic.AccountLongShare.InexactFloat64()
				contractKCandleVo.AccountShortShare = latestStatistic.AccountShortShare.InexactFloat64()
				contractKCandleVo.AccountLongShortRatio = latestStatistic.AccountLongShortRatio.InexactFloat64()
				contractKCandleVo.TopTraderPositionLongShare = latestStatistic.TopTraderPositionLongShare.InexactFloat64()
				contractKCandleVo.TopTraderPositionShortShare = latestStatistic.TopTraderPositionShortShare.InexactFloat64()
				contractKCandleVo.TopTraderPositionLongShortRatio = latestStatistic.TopTraderPositionLongShortRatio.InexactFloat64()
			}
		}

		contractKCandleVos = append(contractKCandleVos, contractKCandleVo)
	}

	return contractKCandleVos
}

// queryReachingBack spans from reach before the first bar to the last bar's close.
func (alignmentDomain ContractKCandleAlignmentDomain) queryReachingBack(reach time.Duration) KCandleQueryDomain {
	firstBucket := alignmentDomain.buckets[0]
	lastBucket := alignmentDomain.buckets[len(alignmentDomain.buckets)-1]

	return KCandleQueryDomain{
		symbol:    firstBucket.Symbol,
		startTime: firstBucket.OpenTime.UTC().Add(-reach),
		endTime:   lastBucket.OpenTime.UTC().Add(alignmentDomain.interval.duration),
	}
}

// readLimitReachingBack is the maximum record count at one per spacing over that stretch, both ends included.
func (alignmentDomain ContractKCandleAlignmentDomain) readLimitReachingBack(
	reach time.Duration, spacing time.Duration,
) int {
	query := alignmentDomain.queryReachingBack(reach)

	return int(query.EndTime().Sub(query.StartTime())/spacing) + 1
}
