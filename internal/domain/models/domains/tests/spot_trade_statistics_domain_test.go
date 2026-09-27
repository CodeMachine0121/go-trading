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

func closedSpotSummary(
	id uint, market string, closedAt time.Time, netProfit string, returnRate float64, rMultiple *float64,
) dto.SpotTradeRecordDto {
	closedAtCopy := closedAt

	return dto.SpotTradeRecordDto{
		ID: id, Market: market, ClosedAt: &closedAtCopy,
		Outcome: dto.SpotTradeOutcomeDto{
			NetProfit: decimal.RequireFromString(netProfit), ReturnRate: &returnRate, RMultiple: rMultiple,
		},
	}
}

func TestSpotTradeStatisticsDomain(t *testing.T) {
	closedAt := time.Date(2026, 9, 20, 5, 0, 0, 0, time.UTC)

	t.Run("each market is summed on its own and never added across currencies", func(t *testing.T) {
		trades := []dto.SpotTradeRecordDto{
			closedSpotSummary(1, "taiwanStock", closedAt.Add(2*time.Hour), "67900", 0.0647, rOf(1.36)),
			closedSpotSummary(2, "taiwanStock", closedAt, "-20000", -0.02, nil),
			closedSpotSummary(3, "crypto", closedAt, "150", 0.03, nil),
		}

		statistics := domains.NewSpotTradeStatisticsDomain("30d", trades).Statistics()

		require.Len(t, statistics.Markets, 2)
		taiwanStock, crypto := statistics.Markets[0], statistics.Markets[1]
		assert.Equal(t, "30d", statistics.Period)
		assert.Equal(t, "taiwanStock", taiwanStock.Market)
		assert.Equal(t, "TWD", taiwanStock.Currency)
		assert.Equal(t, 2, taiwanStock.ClosedTradeCount)
		assert.Equal(t, "47900", taiwanStock.NetProfit.String())
		assert.InDelta(t, 0.5, *taiwanStock.WinRate, 0.0001)
		assert.InDelta(t, 0.02235, *taiwanStock.AverageReturnRate, 0.00001)
		assert.InDelta(t, 3.395, *taiwanStock.ProfitFactor, 0.001)
		assert.Equal(t, "crypto", crypto.Market)
		assert.Equal(t, "USDT", crypto.Currency)
		assert.Equal(t, "150", crypto.NetProfit.String())
		assert.Nil(t, crypto.ProfitFactor)
	})

	t.Run("the cumulative profit follows the order trades closed", func(t *testing.T) {
		trades := []dto.SpotTradeRecordDto{
			closedSpotSummary(1, "taiwanStock", closedAt.Add(2*time.Hour), "67900", 0.0647, nil),
			closedSpotSummary(2, "taiwanStock", closedAt, "-20000", -0.02, nil),
		}

		cumulativeProfit := domains.NewSpotTradeStatisticsDomain("30d", trades).Statistics().Markets[0].CumulativeProfit

		require.Len(t, cumulativeProfit, 2)
		assert.Equal(t, uint(2), cumulativeProfit[0].TradeID)
		assert.Equal(t, "-20000", cumulativeProfit[0].CumulativeNetProfit.String())
		assert.Equal(t, "47900", cumulativeProfit[1].CumulativeNetProfit.String())
	})

	t.Run("the average R stands only on trades with a planned stop and says how many", func(t *testing.T) {
		trades := []dto.SpotTradeRecordDto{
			closedSpotSummary(1, "taiwanStock", closedAt, "100", 0.01, rOf(2)),
			closedSpotSummary(2, "taiwanStock", closedAt, "100", 0.01, rOf(1)),
			closedSpotSummary(3, "taiwanStock", closedAt, "100", 0.01, rOf(0)),
			closedSpotSummary(4, "taiwanStock", closedAt, "100", 0.01, nil),
		}

		taiwanStock := domains.NewSpotTradeStatisticsDomain("30d", trades).Statistics().Markets[0]

		assert.Equal(t, 3, taiwanStock.RTradeCount)
		assert.InDelta(t, 1, *taiwanStock.AverageRMultiple, 0.0001)
	})

	t.Run("mistakes are costed in money and return within the spot journal", func(t *testing.T) {
		chasing := dto.TradeTagDto{ID: 3, Kind: "mistake", Name: "追價進場"}
		first := closedSpotSummary(1, "taiwanStock", closedAt, "-20000", -0.02, nil)
		first.MistakeTags = []dto.TradeTagDto{chasing}
		second := closedSpotSummary(2, "taiwanStock", closedAt, "-10000", -0.04, nil)
		second.MistakeTags = []dto.TradeTagDto{chasing}

		mistakeCosts := domains.NewSpotTradeStatisticsDomain("30d", []dto.SpotTradeRecordDto{first, second}).
			Statistics().Markets[0].MistakeCosts

		require.Len(t, mistakeCosts, 1)
		assert.Equal(t, "追價進場", mistakeCosts[0].Name)
		assert.Equal(t, 2, mistakeCosts[0].TradeCount)
		assert.Equal(t, "-30000", mistakeCosts[0].TotalNetProfit.String())
		assert.InDelta(t, -0.03, *mistakeCosts[0].AverageReturnRate, 0.0001)
	})

	t.Run("returns fall into their bars", func(t *testing.T) {
		trades := []dto.SpotTradeRecordDto{
			closedSpotSummary(1, "crypto", closedAt, "-1", -0.2, nil),
			closedSpotSummary(2, "crypto", closedAt, "-1", -0.05, nil),
			closedSpotSummary(3, "crypto", closedAt, "1", 0.0647, nil),
			closedSpotSummary(4, "crypto", closedAt, "1", 0.3, nil),
		}

		distribution := domains.NewSpotTradeStatisticsDomain("30d", trades).Statistics().Markets[1].ReturnDistribution

		counts := map[string]int{}
		for _, bucket := range distribution {
			counts[bucket.Label] = bucket.Count
		}
		assert.Equal(t, map[string]int{"≤-10%": 1, "-10%~-5%": 1, "-5%~0%": 0, "0%~5%": 0, "5%~10%": 1, ">10%": 1}, counts)
	})

	t.Run("strategy-led and self-judged trades are compared by return", func(t *testing.T) {
		strategyID := uint(11)
		followed := closedSpotSummary(1, "taiwanStock", closedAt, "100", 0.04, nil)
		followed.TradingStrategyID = &strategyID
		selfJudged := closedSpotSummary(2, "taiwanStock", closedAt, "-50", -0.02, nil)

		taiwanStock := domains.NewSpotTradeStatisticsDomain("30d", []dto.SpotTradeRecordDto{followed, selfJudged}).
			Statistics().Markets[0]

		assert.Equal(t, 1, taiwanStock.WithTradingStrategy.TradeCount)
		assert.InDelta(t, 0.04, *taiwanStock.WithTradingStrategy.AverageReturnRate, 0.0001)
		assert.InDelta(t, 0, *taiwanStock.SelfJudged.WinRate, 0.0001)
	})

	t.Run("a market with nothing closed says so rather than showing zero", func(t *testing.T) {
		statistics := domains.NewSpotTradeStatisticsDomain("7d", nil).Statistics()

		for _, market := range statistics.Markets {
			assert.Equal(t, 0, market.ClosedTradeCount)
			assert.Nil(t, market.WinRate)
			assert.Nil(t, market.AverageReturnRate)
			assert.Empty(t, market.CumulativeProfit)
		}
	})
}
