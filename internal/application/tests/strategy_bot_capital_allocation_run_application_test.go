package application_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASymbolNeverBorrowsTheCapitalAnotherLeavesUnused(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectSourcesOn(sourcesBySymbol{
		"BTCUSDT": {vo.SignalHold, vo.SignalHold},
		"ETHUSDT": {vo.SignalBuy, vo.SignalBuy},
	})
	underTest.expectQueuedMessage(func(message string) { assert.Contains(t, message, "開倉金額 30") })
	bot := aDueBotWatching([2]string{"BTCUSDT", ""}, [2]string{"ETHUSDT", ""})
	bot.WatchedSymbols[0].AllocationPercentage = decimal.NewFromInt(30)
	bot.WatchedSymbols[1].AllocationPercentage = decimal.NewFromInt(30)
	bot.PositionPlanCapital = decimal.NewFromInt(100)

	underTest.runsTheBot(t, bot)

	require.Len(t, underTest.queuedMessages(), 1)
}

func TestASymbolHoldingTheWholeCapitalReadsAsItAlwaysDid(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectSourcesOn(sourcesBySymbol{"BTCUSDT": {vo.SignalBuy, vo.SignalBuy}})
	underTest.expectQueuedMessage(func(message string) {
		assert.Contains(t, message, "開倉金額 50")
		assert.NotContains(t, message, "資金分配")
		assert.NotContains(t, message, "分到")
	})
	bot := aDueBotWatching([2]string{"BTCUSDT", ""})
	bot.WatchedSymbols[0].AllocationPercentage = decimal.NewFromInt(100)
	bot.PositionPlanCapital = decimal.NewFromInt(50)
	bot.PositionPlanSizingMode = string(vo.PositionSizingModeFixedAmount)
	bot.PositionPlanSizingValue = decimal.NewFromInt(50)

	underTest.runsTheBot(t, bot)
}

// aContractBotHoldingHalf has 120 of margin in all, half of it BTCUSDT's, at three times leverage.
func aContractBotHoldingHalf() entities.StrategyBot {
	bot := anAutoOrderingContractBot(string(vo.SignalSell))
	bot.PositionPlanCapital = decimal.NewFromInt(120)
	bot.WatchedSymbols[0].AllocationPercentage = decimal.NewFromInt(50)

	return bot
}

func TestAContractSymbolOrdersExactlyWhatItsShareSuggests(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectTheVenue(aContractSpecification(), nil, "")

	underTest.runsAContractRound(t, aContractBotHoldingHalf(), vo.ContractTradingModeLongOnly, vo.SignalBuy, vo.SignalBuy)

	require.Len(t, underTest.queuedMessages(), 1)
	message := underTest.queuedMessages()[0].Text
	assert.Contains(t, message, "資金分配 50%：總金額 120 中的 60")
	assert.Contains(t, message, "數量 0.002")
	require.Len(t, *underTest.queuedAutoOrders, 1)
	queued := (*underTest.queuedAutoOrders)[0]
	require.True(t, queued.OpenQuantity.Valid)
	assertFigure(t, "0.002", queued.OpenQuantity)
}

func TestAContractSymbolsShareIsMarginNotNotional(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectTheVenue(entities.ContractTradingSymbol{}, nil, "")
	bot := aContractBotHoldingHalf()
	bot.PositionPlanCapital = decimal.NewFromInt(100)
	bot.PositionPlanLeverage = decimal.NewFromInt(5)

	underTest.runsAContractRound(t, bot, vo.ContractTradingModeLongOnly, vo.SignalBuy, vo.SignalBuy)

	require.Len(t, underTest.queuedMessages(), 1)
	assert.Contains(t, underTest.queuedMessages()[0].Text, "保證金 50（5 倍槓桿，名目 250）")
}

func TestASymbolWhoseShareCannotCoverTheFixedAmountNeitherSuggestsNorOrders(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	bot := aContractBotHoldingHalf()
	bot.PositionPlanCapital = decimal.NewFromInt(150)
	bot.PositionPlanSizingMode = string(vo.PositionSizingModeFixedAmount)
	bot.PositionPlanSizingValue = decimal.NewFromInt(50)
	bot.WatchedSymbols[0].AllocationPercentage = decimal.RequireFromString("33.33")

	underTest.runsAContractRound(t, bot, vo.ContractTradingModeLongOnly, vo.SignalBuy, vo.SignalBuy)

	require.Len(t, underTest.queuedMessages(), 1)
	message := underTest.queuedMessages()[0].Text
	assert.Contains(t, message, "資金分配 33.33%：總金額 150 中的 49.995")
	assert.Contains(t, message, "這個標的分到的資金不足，押不下 50")
	assert.NotContains(t, message, "保證金 50")
	require.Len(t, *underTest.queuedAutoOrders, 1)
	queued := (*underTest.queuedAutoOrders)[0]
	assert.False(t, queued.OpenQuantity.Valid)
	assert.Equal(t, "這個標的分到的資金不足，押不下 50，不下單", queued.NoOpenReason)
}
