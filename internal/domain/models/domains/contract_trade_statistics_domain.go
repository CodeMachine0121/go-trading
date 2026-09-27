package domains

import (
	"cmp"
	"slices"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// contractTradeRBuckets are the R distribution's bars; the first and last are open-ended.
var contractTradeRBuckets = []struct {
	label      string
	lowerBound float64
	upperBound float64
}{
	{label: "≤-1", lowerBound: -1e18, upperBound: -1},
	{label: "-1~0", lowerBound: -1, upperBound: 0},
	{label: "0~1", lowerBound: 0, upperBound: 1},
	{label: "1~2", lowerBound: 1, upperBound: 2},
	{label: "2~3", lowerBound: 2, upperBound: 3},
	{label: ">3", lowerBound: 3, upperBound: 1e18},
}

// ContractTradeStatisticsDomain sums up closed trades that already carry their outcomes; it never reads anything itself.
type ContractTradeStatisticsDomain struct {
	period string
	trades []dto.ContractTradeRecordDto
}

// NewContractTradeStatisticsDomain orders trades by when they closed, which is the order the cumulative R curve is drawn in.
func NewContractTradeStatisticsDomain(period string, closedTrades []dto.ContractTradeRecordDto) ContractTradeStatisticsDomain {
	orderedTrades := slices.Clone(closedTrades)
	slices.SortStableFunc(orderedTrades, func(earlier, later dto.ContractTradeRecordDto) int {
		return earlier.ClosedAt.Compare(*later.ClosedAt)
	})

	return ContractTradeStatisticsDomain{period: period, trades: orderedTrades}
}

func (statisticsDomain ContractTradeStatisticsDomain) Statistics() dto.ContractTradeStatisticsDto {
	statisticsDto := dto.ContractTradeStatisticsDto{
		Period:           statisticsDomain.period,
		ClosedTradeCount: len(statisticsDomain.trades),
		NetProfit:        decimal.Zero,
		CumulativeR:      []dto.ContractTradeCumulativeRPointDto{},
		MistakeCosts:     []dto.ContractTradeMistakeCostDto{},
	}

	winningProfit := decimal.Zero
	losingLoss := decimal.Zero
	winningGrossProfit := decimal.Zero
	costs := decimal.Zero
	cumulativeRMultiple := 0.0
	slippageTotal := 0.0
	bucketCounts := make([]int, len(contractTradeRBuckets))
	mistakeCostIndexByTag := map[uint]int{}
	tally := NewContractTradeWinTallyDomain(statisticsDomain.trades)

	for _, trade := range statisticsDomain.trades {
		outcome := trade.Outcome
		statisticsDto.NetProfit = statisticsDto.NetProfit.Add(outcome.NetProfit)
		costs = costs.Add(outcome.TotalFee)
		if outcome.Funding.Available && outcome.Funding.Amount.IsNegative() {
			costs = costs.Add(outcome.Funding.Amount.Neg())
		}

		if tally.IsWin(trade) {
			winningProfit = winningProfit.Add(outcome.NetProfit)
			winningGrossProfit = winningGrossProfit.Add(outcome.GrossProfit)
		} else {
			losingLoss = losingLoss.Add(outcome.NetProfit.Neg())
		}

		if outcome.EntrySlippagePercentage != nil {
			slippageTotal += *outcome.EntrySlippagePercentage
			statisticsDto.EntrySlippageTradeCount++
		}

		if outcome.RMultiple == nil {
			statisticsDto.RExcludedCount++
			continue
		}

		rMultiple := *outcome.RMultiple
		cumulativeRMultiple += rMultiple
		statisticsDto.CumulativeR = append(statisticsDto.CumulativeR, dto.ContractTradeCumulativeRPointDto{
			TradeID:             trade.ID,
			ClosedAt:            *trade.ClosedAt,
			RMultiple:           rMultiple,
			CumulativeRMultiple: cumulativeRMultiple,
		})

		for bucketIndex, bucket := range contractTradeRBuckets {
			if rMultiple > bucket.lowerBound && rMultiple <= bucket.upperBound {
				bucketCounts[bucketIndex]++
			}
		}

		for _, mistakeTag := range trade.MistakeTags {
			costIndex, isCounted := mistakeCostIndexByTag[mistakeTag.ID]
			if !isCounted {
				costIndex = len(statisticsDto.MistakeCosts)
				mistakeCostIndexByTag[mistakeTag.ID] = costIndex
				statisticsDto.MistakeCosts = append(statisticsDto.MistakeCosts, dto.ContractTradeMistakeCostDto{
					TagID: mistakeTag.ID, Name: mistakeTag.Name,
				})
			}
			statisticsDto.MistakeCosts[costIndex].TradeCount++
			statisticsDto.MistakeCosts[costIndex].TotalRMultiple += rMultiple
		}
	}

	statisticsDto.WinCount = tally.WinCount()
	statisticsDto.WinRate = tally.WinRate()
	statisticsDto.AverageRMultiple = tally.AverageRMultiple()
	statisticsDto.RDistribution = make([]dto.ContractTradeRBucketDto, 0, len(contractTradeRBuckets))
	for bucketIndex, bucket := range contractTradeRBuckets {
		statisticsDto.RDistribution = append(statisticsDto.RDistribution, dto.ContractTradeRBucketDto{
			Label: bucket.label, Count: bucketCounts[bucketIndex],
		})
	}

	slices.SortStableFunc(statisticsDto.MistakeCosts, func(earlier, later dto.ContractTradeMistakeCostDto) int {
		return cmp.Compare(earlier.TotalRMultiple, later.TotalRMultiple)
	})

	if losingLoss.IsPositive() {
		profitFactor := winningProfit.Div(losingLoss).InexactFloat64()
		statisticsDto.ProfitFactor = &profitFactor
	}

	if winningGrossProfit.IsPositive() {
		feeToGrossProfitRatio := costs.Div(winningGrossProfit).InexactFloat64()
		statisticsDto.FeeToGrossProfitRatio = &feeToGrossProfitRatio
	}

	if statisticsDto.EntrySlippageTradeCount > 0 {
		averageEntrySlippagePercentage := slippageTotal / float64(statisticsDto.EntrySlippageTradeCount)
		statisticsDto.AverageEntrySlippagePercentage = &averageEntrySlippagePercentage
	}

	statisticsDto.WithTradingStrategy = statisticsDomain.groupStatisticsOf(tally.FollowingATradingStrategy(true))
	statisticsDto.SelfJudged = statisticsDomain.groupStatisticsOf(tally.FollowingATradingStrategy(false))

	return statisticsDto
}

func (statisticsDomain ContractTradeStatisticsDomain) groupStatisticsOf(
	tally ContractTradeWinTallyDomain,
) dto.ContractTradeGroupStatisticsDto {
	return dto.ContractTradeGroupStatisticsDto{
		TradeCount:       tally.TradeCount(),
		WinRate:          tally.WinRate(),
		AverageRMultiple: tally.AverageRMultiple(),
	}
}
