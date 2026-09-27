package domains

import (
	"slices"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// spotTradeReturnBuckets are the return distribution's bars; the first and last are open-ended.
var spotTradeReturnBuckets = []struct {
	label      string
	lowerBound float64
	upperBound float64
}{
	{label: "≤-10%", lowerBound: -1e18, upperBound: -0.10},
	{label: "-10%~-5%", lowerBound: -0.10, upperBound: -0.05},
	{label: "-5%~0%", lowerBound: -0.05, upperBound: 0},
	{label: "0%~5%", lowerBound: 0, upperBound: 0.05},
	{label: "5%~10%", lowerBound: 0.05, upperBound: 0.10},
	{label: ">10%", lowerBound: 0.10, upperBound: 1e18},
}

// spotTradeStatisticsMarkets fixes the groups and their order, so a market with nothing closed still shows as such.
var spotTradeStatisticsMarkets = []vo.MarketVo{vo.MarketTaiwanStock, vo.MarketCrypto}

// SpotTradeStatisticsDomain sums up closed spot trades that already carry their outcomes, one group per market.
type SpotTradeStatisticsDomain struct {
	period string
	trades []dto.SpotTradeRecordDto
}

// NewSpotTradeStatisticsDomain orders trades by when they closed, which is the order the cumulative profit is drawn in.
func NewSpotTradeStatisticsDomain(period string, closedTrades []dto.SpotTradeRecordDto) SpotTradeStatisticsDomain {
	orderedTrades := slices.Clone(closedTrades)
	slices.SortStableFunc(orderedTrades, func(earlier, later dto.SpotTradeRecordDto) int {
		return earlier.ClosedAt.Compare(*later.ClosedAt)
	})

	return SpotTradeStatisticsDomain{period: period, trades: orderedTrades}
}

func (statisticsDomain SpotTradeStatisticsDomain) Statistics() dto.SpotTradeStatisticsDto {
	statisticsDto := dto.SpotTradeStatisticsDto{
		Period:  statisticsDomain.period,
		Markets: make([]dto.SpotTradeMarketStatisticsDto, 0, len(spotTradeStatisticsMarkets)),
	}

	for _, market := range spotTradeStatisticsMarkets {
		marketTrades := slices.DeleteFunc(slices.Clone(statisticsDomain.trades), func(trade dto.SpotTradeRecordDto) bool {
			return trade.Market != string(market)
		})
		tally := NewSpotTradeResultsDomain(marketTrades).Tally()
		marketStatistics := dto.SpotTradeMarketStatisticsDto{
			Market:                         string(market),
			Currency:                       NewSpotTradeMarketDomain(string(market)).Currency(),
			ClosedTradeCount:               tally.TradeCount(),
			WinCount:                       tally.WinCount(),
			WinRate:                        tally.WinRate(),
			NetProfit:                      decimal.Zero,
			AverageReturnRate:              tally.AverageReturnRate(),
			AverageRMultiple:               tally.AverageRMultiple(),
			RTradeCount:                    tally.RMultipleTradeCount(),
			CumulativeProfit:               []dto.SpotTradeCumulativeProfitPointDto{},
			MistakeCosts:                   []dto.SpotTradeMistakeCostDto{},
			AverageEntrySlippagePercentage: tally.AverageEntrySlippagePercentage(),
			EntrySlippageTradeCount:        tally.EntrySlippageTradeCount(),
			WithTradingStrategy:            statisticsDomain.groupStatisticsOf(tally.FollowingATradingStrategy(true)),
			SelfJudged:                     statisticsDomain.groupStatisticsOf(tally.FollowingATradingStrategy(false)),
		}

		winningProfit := decimal.Zero
		losingLoss := decimal.Zero
		bucketCounts := make([]int, len(spotTradeReturnBuckets))
		mistakeTradesByTag := map[uint][]dto.SpotTradeRecordDto{}

		for _, trade := range marketTrades {
			outcome := trade.Outcome
			marketStatistics.NetProfit = marketStatistics.NetProfit.Add(outcome.NetProfit)
			marketStatistics.CumulativeProfit = append(marketStatistics.CumulativeProfit, dto.SpotTradeCumulativeProfitPointDto{
				TradeID:             trade.ID,
				ClosedAt:            *trade.ClosedAt,
				NetProfit:           outcome.NetProfit,
				CumulativeNetProfit: marketStatistics.NetProfit,
			})

			if tally.IsWin(vo.TradeResultVo{NetProfit: outcome.NetProfit}) {
				winningProfit = winningProfit.Add(outcome.NetProfit)
			} else {
				losingLoss = losingLoss.Add(outcome.NetProfit.Neg())
			}

			if outcome.ReturnRate != nil {
				for bucketIndex, bucket := range spotTradeReturnBuckets {
					if *outcome.ReturnRate > bucket.lowerBound && *outcome.ReturnRate <= bucket.upperBound {
						bucketCounts[bucketIndex]++
					}
				}
			}

			for _, mistakeTag := range trade.MistakeTags {
				if _, isCounted := mistakeTradesByTag[mistakeTag.ID]; !isCounted {
					marketStatistics.MistakeCosts = append(marketStatistics.MistakeCosts, dto.SpotTradeMistakeCostDto{
						TagID: mistakeTag.ID, Name: mistakeTag.Name, TotalNetProfit: decimal.Zero,
					})
				}
				mistakeTradesByTag[mistakeTag.ID] = append(mistakeTradesByTag[mistakeTag.ID], trade)
			}
		}

		for costIndex, mistakeCost := range marketStatistics.MistakeCosts {
			taggedTrades := mistakeTradesByTag[mistakeCost.TagID]
			for _, trade := range taggedTrades {
				marketStatistics.MistakeCosts[costIndex].TotalNetProfit =
					marketStatistics.MistakeCosts[costIndex].TotalNetProfit.Add(trade.Outcome.NetProfit)
			}
			marketStatistics.MistakeCosts[costIndex].TradeCount = len(taggedTrades)
			marketStatistics.MistakeCosts[costIndex].AverageReturnRate = NewSpotTradeResultsDomain(taggedTrades).Tally().AverageReturnRate()
		}
		slices.SortStableFunc(marketStatistics.MistakeCosts, func(earlier, later dto.SpotTradeMistakeCostDto) int {
			return earlier.TotalNetProfit.Cmp(later.TotalNetProfit)
		})

		marketStatistics.ReturnDistribution = make([]dto.SpotTradeReturnBucketDto, 0, len(spotTradeReturnBuckets))
		for bucketIndex, bucket := range spotTradeReturnBuckets {
			marketStatistics.ReturnDistribution = append(marketStatistics.ReturnDistribution, dto.SpotTradeReturnBucketDto{
				Label: bucket.label, Count: bucketCounts[bucketIndex],
			})
		}

		if losingLoss.IsPositive() {
			profitFactor := winningProfit.Div(losingLoss).InexactFloat64()
			marketStatistics.ProfitFactor = &profitFactor
		}

		statisticsDto.Markets = append(statisticsDto.Markets, marketStatistics)
	}

	return statisticsDto
}

func (statisticsDomain SpotTradeStatisticsDomain) groupStatisticsOf(tally TradeWinTallyDomain) dto.SpotTradeGroupStatisticsDto {
	return dto.SpotTradeGroupStatisticsDto{
		TradeCount:        tally.TradeCount(),
		WinRate:           tally.WinRate(),
		AverageReturnRate: tally.AverageReturnRate(),
	}
}
