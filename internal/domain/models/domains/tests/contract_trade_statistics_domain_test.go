package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type closedTradeShape struct {
	netProfit         string
	grossProfit       string
	totalFee          string
	fundingAmount     string
	rMultiple         *float64
	tradingStrategyID *uint
	mistakeTags       []dto.TradeTagDto
	slippage          *float64
}

func rOf(value float64) *float64 {
	return &value
}

func closedTradesOf(shapes ...closedTradeShape) []dto.ContractTradeRecordDto {
	trades := []dto.ContractTradeRecordDto{}
	for index, shape := range shapes {
		closedAt := tradeOpenedAt.Add(time.Duration(index) * time.Hour)
		grossProfit := shape.grossProfit
		if grossProfit == "" {
			grossProfit = shape.netProfit
		}
		totalFee := shape.totalFee
		if totalFee == "" {
			totalFee = "0"
		}
		fundingAmount := shape.fundingAmount
		if fundingAmount == "" {
			fundingAmount = "0"
		}

		trades = append(trades, dto.ContractTradeRecordDto{
			ID: uint(index + 1), ClosedAt: &closedAt, TradingStrategyID: shape.tradingStrategyID,
			MistakeTags: shape.mistakeTags,
			Outcome: dto.ContractTradeOutcomeDto{
				NetProfit:   decimal.RequireFromString(shape.netProfit),
				GrossProfit: decimal.RequireFromString(grossProfit),
				TotalFee:    decimal.RequireFromString(totalFee),
				Funding: dto.ContractTradeFundingDto{
					Available: true, Amount: decimal.RequireFromString(fundingAmount),
				},
				RMultiple:               shape.rMultiple,
				EntrySlippagePercentage: shape.slippage,
			},
		})
	}

	return trades
}

func TestContractTradeStatisticsDomainWinRateAndR(t *testing.T) {
	t.Run("thirty trades of which fourteen won", func(t *testing.T) {
		shapes := []closedTradeShape{}
		for index := 0; index < 30; index++ {
			if index < 14 {
				shapes = append(shapes, closedTradeShape{netProfit: "10", rMultiple: rOf(1)})
				continue
			}
			shapes = append(shapes, closedTradeShape{netProfit: "-5", rMultiple: rOf(-0.5)})
		}

		statistics := domains.NewContractTradeStatisticsDomain("30d", closedTradesOf(shapes...)).Statistics()

		assert.Equal(t, 30, statistics.ClosedTradeCount)
		require.NotNil(t, statistics.WinRate)
		assert.InDelta(t, 0.47, *statistics.WinRate, 0.005)
		assert.InDelta(t, (14.0-8.0)/30.0, *statistics.AverageRMultiple, 0.0001)
		assert.InDelta(t, 140.0/80.0, *statistics.ProfitFactor, 0.0001)
		require.Len(t, statistics.CumulativeR, 30)
		assert.InDelta(t, 6.0, statistics.CumulativeR[29].CumulativeRMultiple, 0.0001)
		assert.Equal(t, "30d", statistics.Period)
	})

	t.Run("breaking even does not count as a win", func(t *testing.T) {
		statistics := domains.NewContractTradeStatisticsDomain("30d", closedTradesOf(
			closedTradeShape{netProfit: "10"}, closedTradeShape{netProfit: "0"})).Statistics()

		assert.InDelta(t, 0.5, *statistics.WinRate, 0.0001)
	})

	t.Run("trades without a stop count for money but not for R", func(t *testing.T) {
		statistics := domains.NewContractTradeStatisticsDomain("30d", closedTradesOf(
			closedTradeShape{netProfit: "10", rMultiple: rOf(2)},
			closedTradeShape{netProfit: "-5"},
			closedTradeShape{netProfit: "3"},
		)).Statistics()

		assert.Equal(t, 3, statistics.ClosedTradeCount)
		assert.InDelta(t, 2.0/3.0, *statistics.WinRate, 0.0001)
		assert.InDelta(t, 2.0, *statistics.AverageRMultiple, 0.0001)
		assert.Equal(t, 2, statistics.RExcludedCount)
		assert.Len(t, statistics.CumulativeR, 1)
	})

	t.Run("nothing closed is not a zero percent win rate", func(t *testing.T) {
		statistics := domains.NewContractTradeStatisticsDomain("7d", nil).Statistics()

		assert.Equal(t, 0, statistics.ClosedTradeCount)
		assert.Nil(t, statistics.WinRate)
		assert.Nil(t, statistics.AverageRMultiple)
		assert.Nil(t, statistics.ProfitFactor)
		assert.Nil(t, statistics.FeeToGrossProfitRatio)
		assert.Nil(t, statistics.AverageEntrySlippagePercentage)
		assert.Empty(t, statistics.CumulativeR)
		assert.Len(t, statistics.RDistribution, 6)
	})
}

func TestContractTradeStatisticsDomainBreakdowns(t *testing.T) {
	strategyID := uint(12)
	movedStop := dto.TradeTagDto{ID: 2, Kind: "mistake", Name: "移動止損"}
	chased := dto.TradeTagDto{ID: 1, Kind: "mistake", Name: "追價進場"}

	t.Run("each mistake adds up the R it cost", func(t *testing.T) {
		statistics := domains.NewContractTradeStatisticsDomain("30d", closedTradesOf(
			closedTradeShape{netProfit: "-10", rMultiple: rOf(-1), mistakeTags: []dto.TradeTagDto{movedStop}},
			closedTradeShape{netProfit: "-10", rMultiple: rOf(-0.8), mistakeTags: []dto.TradeTagDto{movedStop}},
			closedTradeShape{netProfit: "-10", rMultiple: rOf(-0.7), mistakeTags: []dto.TradeTagDto{movedStop, chased}},
			closedTradeShape{netProfit: "-10", rMultiple: rOf(-0.7), mistakeTags: []dto.TradeTagDto{movedStop}},
			closedTradeShape{netProfit: "-10", mistakeTags: []dto.TradeTagDto{chased}},
		)).Statistics()

		require.Len(t, statistics.MistakeCosts, 2)
		assert.Equal(t, "移動止損", statistics.MistakeCosts[0].Name)
		assert.Equal(t, uint(2), statistics.MistakeCosts[0].TagID)
		assert.Equal(t, 4, statistics.MistakeCosts[0].TradeCount)
		assert.InDelta(t, -3.2, statistics.MistakeCosts[0].TotalRMultiple, 0.0001)
		assert.Equal(t, "追價進場", statistics.MistakeCosts[1].Name)
		assert.Equal(t, 1, statistics.MistakeCosts[1].TradeCount)
	})

	t.Run("mistakes that cost the same keep the order they first appeared in", func(t *testing.T) {
		statistics := domains.NewContractTradeStatisticsDomain("30d", closedTradesOf(
			closedTradeShape{netProfit: "-10", rMultiple: rOf(-1), mistakeTags: []dto.TradeTagDto{chased}},
			closedTradeShape{netProfit: "-10", rMultiple: rOf(-1), mistakeTags: []dto.TradeTagDto{movedStop}},
			closedTradeShape{netProfit: "10", rMultiple: rOf(0.5), mistakeTags: []dto.TradeTagDto{{ID: 3, Name: "提早出場"}}},
		)).Statistics()

		require.Len(t, statistics.MistakeCosts, 3)
		assert.Equal(t, []string{"追價進場", "移動止損", "提早出場"}, []string{
			statistics.MistakeCosts[0].Name, statistics.MistakeCosts[1].Name, statistics.MistakeCosts[2].Name})
	})

	t.Run("trades following a strategy are told apart from self-judged ones", func(t *testing.T) {
		statistics := domains.NewContractTradeStatisticsDomain("30d", closedTradesOf(
			closedTradeShape{netProfit: "10", rMultiple: rOf(1), tradingStrategyID: &strategyID},
			closedTradeShape{netProfit: "-5", rMultiple: rOf(-1), tradingStrategyID: &strategyID},
			closedTradeShape{netProfit: "-5", rMultiple: rOf(-1)},
		)).Statistics()

		assert.Equal(t, 2, statistics.WithTradingStrategy.TradeCount)
		assert.InDelta(t, 0.5, *statistics.WithTradingStrategy.WinRate, 0.0001)
		assert.InDelta(t, 0.0, *statistics.WithTradingStrategy.AverageRMultiple, 0.0001)
		assert.Equal(t, 1, statistics.SelfJudged.TradeCount)
		assert.InDelta(t, 0.0, *statistics.SelfJudged.WinRate, 0.0001)
	})

	t.Run("the average slippage counts only trades started from a bot round", func(t *testing.T) {
		statistics := domains.NewContractTradeStatisticsDomain("30d", closedTradesOf(
			closedTradeShape{netProfit: "10", slippage: rOf(0.08)},
			closedTradeShape{netProfit: "10", slippage: rOf(0.06)},
			closedTradeShape{netProfit: "10"},
		)).Statistics()

		assert.Equal(t, 2, statistics.EntrySlippageTradeCount)
		assert.InDelta(t, 0.07, *statistics.AverageEntrySlippagePercentage, 0.0001)
	})

	t.Run("fees and paid funding against the winners' gross profit", func(t *testing.T) {
		statistics := domains.NewContractTradeStatisticsDomain("30d", closedTradesOf(
			closedTradeShape{netProfit: "880", grossProfit: "1000", totalFee: "100", fundingAmount: "-20"},
			closedTradeShape{netProfit: "-70", grossProfit: "-10", totalFee: "50", fundingAmount: "-10"},
			closedTradeShape{netProfit: "5", grossProfit: "0", totalFee: "0", fundingAmount: "5"},
		)).Statistics()

		require.NotNil(t, statistics.FeeToGrossProfitRatio)
		assert.InDelta(t, 0.18, *statistics.FeeToGrossProfitRatio, 0.0001)
		assert.Equal(t, "815", statistics.NetProfit.String())
	})

	t.Run("each R lands in its bar", func(t *testing.T) {
		statistics := domains.NewContractTradeStatisticsDomain("all", closedTradesOf(
			closedTradeShape{netProfit: "-1", rMultiple: rOf(-1.5)},
			closedTradeShape{netProfit: "-1", rMultiple: rOf(-1)},
			closedTradeShape{netProfit: "-1", rMultiple: rOf(-0.2)},
			closedTradeShape{netProfit: "1", rMultiple: rOf(0.5)},
			closedTradeShape{netProfit: "1", rMultiple: rOf(1.53)},
			closedTradeShape{netProfit: "1", rMultiple: rOf(2.4)},
			closedTradeShape{netProfit: "1", rMultiple: rOf(4)},
		)).Statistics()

		counts := map[string]int{}
		for _, bucket := range statistics.RDistribution {
			counts[bucket.Label] = bucket.Count
		}
		assert.Equal(t, map[string]int{"≤-1": 2, "-1~0": 1, "0~1": 1, "1~2": 1, "2~3": 1, ">3": 1}, counts)
	})
}
