package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestTradeStatisticsDomain is what a replay's finished round trips say about a
// short-term strategy: whether the winners outweigh the losers, what one trade is
// worth on average, how long a trade is held, how long a losing streak gets, and how
// much of what the trades made the charges for trading took.
//
// It reads finished round trips only. A position still open at the end has no exit,
// so it has neither a net profit nor a holding time to contribute.
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
