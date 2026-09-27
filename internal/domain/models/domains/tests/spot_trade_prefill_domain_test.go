package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func spotBuyRound() dto.JournalLinkRoundDto {
	return dto.JournalLinkRoundDto{
		StrategyBotID: 3, StrategyBotName: "台積電波段", Symbol: "2330", MarketDataKind: "kCandle",
		TradingStrategyID: 11, RunNumber: 88, RanAt: time.Date(2026, 9, 24, 1, 0, 0, 0, time.UTC), Result: "buy",
		ReferencePrice: price("1050"), SuggestedStake: price("105000"),
		SuggestedStopLossPrice: price("1000"), SuggestedTakeProfitPrice: price("1150"),
	}
}

func TestSpotTradePrefillDomain(t *testing.T) {
	heldTrade := entities.SpotTradeRecord{
		ID: 5, Market: "taiwanStock", Status: "open",
		Fills: []entities.SpotTradeFill{spotBuy(1, spotBoughtAt, "1050", "1000"), spotSell(2, spotBoughtAt.Add(time.Hour), "1100", "400")},
	}

	t.Run("a buy prefills a new trade with the round's suggestion", func(t *testing.T) {
		prefill := domains.NewSpotTradePrefillDomain(spotBuyRound(), "taiwanStock").Prefill(entities.SpotTradeRecord{}, false)

		assert.Equal(t, "newTrade", prefill.Mode)
		assert.Nil(t, prefill.TargetTradeID)
		assert.Equal(t, "buy", prefill.Signal)
		assert.Equal(t, "2330", prefill.Symbol)
		assert.Equal(t, "taiwanStock", prefill.Market)
		assert.Equal(t, "1050", prefill.Price.Decimal.String())
		assert.Equal(t, "100", prefill.Quantity.Decimal.String())
		assert.Equal(t, "1000", prefill.PlannedStopLossPrice.Decimal.String())
		assert.Equal(t, "1150", prefill.PlannedTakeProfitPrice.Decimal.String())
		assert.Equal(t, uint(11), *prefill.TradingStrategyID)
		assert.True(t, prefill.PriceNeedsConfirmation)
		assert.Equal(t, 88, prefill.RunNumber)
		assert.Empty(t, prefill.MissingReferenceReason)
	})

	t.Run("a buy without a suggestion prefills the symbol and price but no quantity", func(t *testing.T) {
		round := spotBuyRound()
		round.SuggestedStake = decimal.NullDecimal{}
		round.SuggestedStopLossPrice = decimal.NullDecimal{}
		round.SuggestedTakeProfitPrice = decimal.NullDecimal{}

		prefill := domains.NewSpotTradePrefillDomain(round, "taiwanStock").Prefill(entities.SpotTradeRecord{}, false)

		assert.Equal(t, "1050", prefill.Price.Decimal.String())
		assert.False(t, prefill.Quantity.Valid)
		assert.False(t, prefill.PlannedStopLossPrice.Valid)
	})

	t.Run("a buy while already holding adds to that trade", func(t *testing.T) {
		prefill := domains.NewSpotTradePrefillDomain(spotBuyRound(), "taiwanStock").Prefill(heldTrade, true)

		assert.Equal(t, "addBuyFill", prefill.Mode)
		require.NotNil(t, prefill.TargetTradeID)
		assert.Equal(t, uint(5), *prefill.TargetTradeID)
	})

	t.Run("an exit while holding sells everything still held", func(t *testing.T) {
		round := spotBuyRound()
		round.Result = "sell"

		prefill := domains.NewSpotTradePrefillDomain(round, "taiwanStock").Prefill(heldTrade, true)

		assert.Equal(t, "addSellFill", prefill.Mode)
		assert.Equal(t, "sell", prefill.Signal)
		assert.Equal(t, uint(5), *prefill.TargetTradeID)
		assert.Equal(t, "600", prefill.Quantity.Decimal.String())
		assert.Equal(t, "1050", prefill.Price.Decimal.String())
		assert.False(t, prefill.PlannedStopLossPrice.Valid)
	})

	t.Run("an exit with nothing held says so", func(t *testing.T) {
		round := spotBuyRound()
		round.Result = "sell"

		prefill := domains.NewSpotTradePrefillDomain(round, "taiwanStock").Prefill(entities.SpotTradeRecord{}, false)

		assert.Equal(t, "noOpenHolding", prefill.Mode)
		assert.Nil(t, prefill.TargetTradeID)
		assert.False(t, prefill.Quantity.Valid)
	})

	t.Run("a round from before reference prices were kept says so and prefills no quantity", func(t *testing.T) {
		round := spotBuyRound()
		round.ReferencePrice = decimal.NullDecimal{}

		prefill := domains.NewSpotTradePrefillDomain(round, "crypto").Prefill(entities.SpotTradeRecord{}, false)

		assert.Equal(t, "roundPredatesReferencePrices", prefill.MissingReferenceReason)
		assert.False(t, prefill.Quantity.Valid)
		assert.Equal(t, "crypto", prefill.Market)
	})
}

func TestSpotTradeLiveComparisonDomain(t *testing.T) {
	strategyID := uint(11)
	firstClose := time.Date(2026, 9, 10, 5, 0, 0, 0, time.UTC)
	trade := func(id uint, symbol string, openedAt time.Time, closedAt time.Time, netProfit string, slippage *float64) dto.SpotTradeRecordDto {
		closedAtCopy := closedAt
		return dto.SpotTradeRecordDto{
			ID: id, Symbol: symbol, Market: "taiwanStock", TradingStrategyID: &strategyID, OpenedAt: openedAt, ClosedAt: &closedAtCopy,
			Outcome: dto.SpotTradeOutcomeDto{NetProfit: decimal.RequireFromString(netProfit), EntrySlippagePercentage: slippage},
		}
	}

	trades := []dto.SpotTradeRecordDto{
		trade(1, "2330", firstClose.Add(-24*time.Hour), firstClose, "100", rOf(0.5)),
		trade(2, "2330", firstClose.Add(-48*time.Hour), firstClose.Add(-time.Hour), "-50", nil),
		trade(4, "2330", firstClose.Add(24*time.Hour), firstClose.Add(72*time.Hour), "30", nil),
		trade(3, "0050", firstClose, firstClose.Add(time.Hour), "10", rOf(0.1)),
	}

	t.Run("each symbol is replayed over its own stretch without costs", func(t *testing.T) {
		plan := domains.NewSpotTradeLiveComparisonDomain(trades).Plan()

		require.Len(t, plan.Groups, 2)
		group := plan.Groups[1]
		assert.Equal(t, "2330", group.Symbol)
		assert.Equal(t, "taiwanStock", group.Market)
		assert.True(t, group.BacktestRequest.StartTime.Equal(firstClose.Add(-48*time.Hour)))
		assert.True(t, group.BacktestRequest.EndTime.Equal(firstClose.Add(72*time.Hour)))
		assert.Equal(t, "10000", group.BacktestRequest.InitialCapital.String())
		assert.Equal(t, "allIn", group.BacktestRequest.PositionSizingMode)
		assert.True(t, group.BacktestRequest.EntryCostPercentage.IsZero())
		assert.True(t, group.BacktestRequest.ExitCostPercentage.IsZero())
		assert.Equal(t, 3, group.Live.ClosedTradeCount)
		assert.InDelta(t, 2.0/3.0, *group.Live.WinRate, 0.0001)
		assert.InDelta(t, 0.5, *group.Live.AverageEntrySlippagePercentage, 0.0001)
		assert.InDelta(t, 0.3, *plan.AverageEntrySlippagePercentage, 0.0001)
		assert.Equal(t, 2, plan.EntrySlippageTradeCount)
	})

	t.Run("replays stand beside live figures and a failed one says why", func(t *testing.T) {
		plan := domains.NewSpotTradeLiveComparisonDomain(trades).Plan()
		winRate := 0.6

		rows := domains.NewSpotTradeLiveComparisonDomain(nil).Compose(plan.Groups, []dto.SpotTradeBacktestAttemptDto{
			{FailureReason: "K 線不夠"},
			{Result: dto.BacktestResultDto{Summary: dto.BacktestSummaryDto{WinRate: &winRate},
				ClosedTrades: []dto.ClosedTradeDto{{}, {}, {}}}},
		})

		require.Len(t, rows, 2)
		assert.Nil(t, rows[0].Backtest)
		assert.Equal(t, "K 線不夠", rows[0].BacktestUnavailableReason)
		require.NotNil(t, rows[1].Backtest)
		assert.Equal(t, 3, rows[1].Backtest.ClosedTradeCount)
		assert.InDelta(t, 0.6, *rows[1].Backtest.WinRate, 0.0001)
		assert.Equal(t, 3, rows[1].Live.ClosedTradeCount)
	})

	t.Run("a deleted strategy keeps its live figures", func(t *testing.T) {
		rows := domains.NewSpotTradeLiveComparisonDomain(nil).ComposeForDeletedTradingStrategy(
			domains.NewSpotTradeLiveComparisonDomain(trades).Plan().Groups)

		for _, row := range rows {
			assert.Equal(t, "交易策略已刪除，無法重演", row.BacktestUnavailableReason)
			assert.Nil(t, row.Backtest)
		}
	})
}
