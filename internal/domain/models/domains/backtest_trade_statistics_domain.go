package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestTradeStatisticsDomain derives short-term statistics (profit factor, expectancy, holding time, loss streak, cost ratio) from finished round trips only.
type BacktestTradeStatisticsDomain struct {
	outcomes []vo.TradeOutcomeVo
}

func NewBacktestTradeStatisticsDomain(outcomes []vo.TradeOutcomeVo) BacktestTradeStatisticsDomain {
	return BacktestTradeStatisticsDomain{outcomes: outcomes}
}

func (statisticsDomain BacktestTradeStatisticsDomain) ToDto() dto.BacktestTradeStatisticsDto {
	statisticsDto := dto.BacktestTradeStatisticsDto{}
	if len(statisticsDomain.outcomes) == 0 {
		return statisticsDto
	}

	wonTotal := decimal.Zero
	lostTotal := decimal.Zero
	netTotal := decimal.Zero
	grossTotal := decimal.Zero
	costTotal := decimal.Zero
	holdingSecondsTotal := int64(0)
	consecutiveLossCount := 0

	for _, outcome := range statisticsDomain.outcomes {
		netTotal = netTotal.Add(outcome.NetProfit)
		grossTotal = grossTotal.Add(outcome.GrossProfit)
		costTotal = costTotal.Add(outcome.TransactionCost)
		holdingSecondsTotal += int64(outcome.ExitTime.Sub(outcome.EntryTime).Seconds())

		if outcome.NetProfit.IsPositive() {
			wonTotal = wonTotal.Add(outcome.NetProfit)
		}
		if outcome.NetProfit.IsNegative() {
			lostTotal = lostTotal.Add(outcome.NetProfit.Abs())
			consecutiveLossCount++
			statisticsDto.MaximumConsecutiveLossCount = max(
				statisticsDto.MaximumConsecutiveLossCount, consecutiveLossCount)
		} else {
			consecutiveLossCount = 0
		}
	}

	tradeCount := int64(len(statisticsDomain.outcomes))
	statisticsDto.Expectancy = decimal.NewNullDecimal(netTotal.Div(decimal.NewFromInt(tradeCount)))
	averageHoldingSeconds := holdingSecondsTotal / tradeCount
	statisticsDto.AverageHoldingSeconds = &averageHoldingSeconds

	if lostTotal.IsPositive() {
		profitFactor, _ := wonTotal.Div(lostTotal).Float64()
		statisticsDto.ProfitFactor = &profitFactor
	}

	if grossTotal.IsPositive() {
		costToGrossProfitRatio, _ := costTotal.Div(grossTotal).Float64()
		statisticsDto.CostToGrossProfitRatio = &costToGrossProfitRatio
	}

	return statisticsDto
}
