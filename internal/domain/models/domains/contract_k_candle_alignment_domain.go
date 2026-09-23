package domains

import (
	"slices"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ContractKCandleAlignmentDomain is the stretch of perpetual contract bars a contract
// script is about to be fed, and every rule that decides what each bar carries.
//
// The bars themselves are already settled — merged contract K candles, one per
// finished bucket, oldest first. What this adds is the two records that are not
// candles: funding settlements, a few a day, and position statistics, one every five
// minutes. It says which stretch of each has to be read, and then lines them up.
//
// **Every figure on a bar comes from before that bar closed.** A settlement or a
// statistic stamped exactly at the close belongs to the next bar, so a script can
// never act on something that had not happened yet when the bar it is looking at was
// finished.
//
// **A figure that is not there is zero** — see ContractKCandleVo. That was chosen by
// the person writing the scripts: it is the easiest to write against, and the script
// carries the knowledge that a zero open interest most likely means "not recorded".
type ContractKCandleAlignmentDomain struct {
	interval AggregationIntervalDomain
	// buckets are the merged contract K candles, oldest first, never empty: a
	// calculation with nothing to feed a script is refused before one of these is
	// built.
	buckets []dto.KCandleContractDto
}

// Buckets that have no contract K candle are not here, and that is the rule rather
// than a gap: which bars exist is decided by the contract K candles alone, whatever
// settlements or statistics that stretch holds.
func newContractKCandleAlignmentDomain(
	interval AggregationIntervalDomain, buckets []dto.KCandleContractDto,
) ContractKCandleAlignmentDomain {
	return ContractKCandleAlignmentDomain{interval: interval, buckets: buckets}
}

// SettlementLeadInCutoff is the moment the first bar opens. The latest settlement
// before it is the rate already in force when the stretch begins, and it has to be
// asked for on its own: it may lie any distance back — a fetch that stopped for a day
// leaves a day between two stored settlements — and no fixed reach backwards is sure
// to catch it.
func (alignmentDomain ContractKCandleAlignmentDomain) SettlementLeadInCutoff() time.Time {
	return alignmentDomain.buckets[0].OpenTime.UTC()
}

// SettlementQuery is the stretch of funding settlements the bars themselves cover: from
// where the first bar opens up to where the last one closes.
func (alignmentDomain ContractKCandleAlignmentDomain) SettlementQuery() KCandleQueryDomain {
	return alignmentDomain.queryReachingBack(0)
}

// SettlementReadLimit is the most settlements that stretch can hold. No contract
// settles more often than hourly, so one per hour, plus the one at either end, is an
// upper bound the stored settlements cannot exceed.
func (alignmentDomain ContractKCandleAlignmentDomain) SettlementReadLimit() int {
	return alignmentDomain.readLimitReachingBack(0, time.Hour)
}

// StatisticQuery is the stretch of position statistics the bars can draw on: from one
// statistic interval before the first bar — the furthest back a bar finer than that
// interval may reach — up to where the last bar closes.
func (alignmentDomain ContractKCandleAlignmentDomain) StatisticQuery() KCandleQueryDomain {
	return alignmentDomain.queryReachingBack(ContractPositionStatisticInterval)
}

// StatisticReadLimit is the most position statistics that stretch can hold: one per
// statistic interval, plus the one at either end.
func (alignmentDomain ContractKCandleAlignmentDomain) StatisticReadLimit() int {
	return alignmentDomain.readLimitReachingBack(ContractPositionStatisticInterval, ContractPositionStatisticInterval)
}

// Aligning hands back the bars the script sees, oldest first, with the funding and
// positioning each one carries.
//
// For every bar:
//
//   - **The funding rate is the one in force at its close**: the rate of the latest
//     settlement strictly before the close. Between two settlements every bar carries
//     the earlier one's rate, because that is the rate that was in force. With several
//     settlements inside one bar — a day, for a symbol settling every eight hours —
//     the latest of them is the one in force at the close.
//   - **Whether it settled** is whether any settlement falls inside the bar, its open
//     included and its close not. Only the latest one before the close has to be
//     asked: if any settlement is inside the bar, the latest one is.
//   - **The position statistic is the latest one strictly before the close, if it is
//     recent enough**: inside the bar, or — for a bar finer than the statistic
//     interval — within one statistic interval of its close. Statistics come every
//     five minutes, so a one-minute bar carries the last one taken; one further back
//     than that means recording stopped, and a bar carrying it would be passing an old
//     state off as the current one. Such a bar carries zeros instead.
//
// The settlements handed in are those the bars cover plus, when there is one, the
// latest before the first bar — see SettlementLeadInCutoff. Settlements and statistics
// may arrive in any order; each is walked once.
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

// queryReachingBack is the stretch from the given reach before the first bar to the
// close of the last one. Both record kinds are read the same way; only how far back
// each has to reach differs.
func (alignmentDomain ContractKCandleAlignmentDomain) queryReachingBack(reach time.Duration) KCandleQueryDomain {
	firstBucket := alignmentDomain.buckets[0]
	lastBucket := alignmentDomain.buckets[len(alignmentDomain.buckets)-1]

	return KCandleQueryDomain{
		symbol:    firstBucket.Symbol,
		startTime: firstBucket.OpenTime.UTC().Add(-reach),
		endTime:   lastBucket.OpenTime.UTC().Add(alignmentDomain.interval.duration),
	}
}

// readLimitReachingBack is how many records at most one every spacing can put in the
// stretch queryReachingBack reads, both ends included.
func (alignmentDomain ContractKCandleAlignmentDomain) readLimitReachingBack(
	reach time.Duration, spacing time.Duration,
) int {
	query := alignmentDomain.queryReachingBack(reach)

	return int(query.EndTime().Sub(query.StartTime())/spacing) + 1
}
