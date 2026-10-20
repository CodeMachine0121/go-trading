package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// aBotWatching saves a bot watching the given symbols, in that order.
func aBotWatching(t *testing.T, database *gorm.DB, name string, symbols ...string) entities.StrategyBot {
	t.Helper()

	bot := aBotRow(name)
	bot.WatchedSymbols = []entities.StrategyBotWatchedSymbol{}
	for _, symbol := range symbols {
		bot.WatchedSymbols = append(bot.WatchedSymbols, entities.StrategyBotWatchedSymbol{Symbol: symbol})
	}

	savedBot, saveError := persistence.NewStrategyBotRepository(database).Save(t.Context(), bot)
	require.NoError(t, saveError)

	return savedBot
}

func TestStrategyBotRepositoryRewriteKeepsTheSymbolsStillListedAsTheyWere(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)
	savedBot := aBotWatching(t, database, "多標的", "BTCUSDT", "ETHUSDT")
	require.Equal(t, []string{"BTCUSDT", "ETHUSDT"}, savedBot.Symbols())

	roundDueAt := time.Date(2026, 10, 20, 6, 0, 0, 0, time.UTC)
	savedBot.WatchedSymbols[0].LastSentSignal = string(vo.SignalBuy)
	savedBot.WatchedSymbols[0].LastSentRoundDueAt = &roundDueAt
	savedBot.WatchedSymbols[1].Conflicting = true
	savedBot.WatchedSymbols[1].HaltReason = string(vo.StrategyBotHaltScriptFailed)
	require.NoError(t, repository.UpdateRunState(t.Context(), savedBot))
	require.NoError(t, repository.UpdateAutoOrderPosition(t.Context(), savedBot.ID, "BTCUSDT", vo.AutoOrderPositionVo{
		Direction: vo.TargetPositionLong, Quantity: decimal.RequireFromString("0.01"),
		StopLossClientID: "gt-ao-1-sl", TakeProfitClientID: "gt-ao-1-tp",
	}))

	rewritten := aBotRow("多標的")
	rewritten.ID = savedBot.ID
	rewritten.WatchedSymbols = []entities.StrategyBotWatchedSymbol{{Symbol: "SOLUSDT"}, {Symbol: "BTCUSDT"}}
	rewrittenBot, rewriteError := repository.Save(t.Context(), rewritten)
	require.NoError(t, rewriteError)

	// Read in the order they were added, so the kept symbol stays first.
	assert.Equal(t, []string{"BTCUSDT", "SOLUSDT"}, rewrittenBot.Symbols())
	bitcoin, _ := rewrittenBot.WatchedSymbol("BTCUSDT")
	assert.Equal(t, string(vo.SignalBuy), bitcoin.LastSentSignal)
	require.NotNil(t, bitcoin.LastSentRoundDueAt)
	assert.True(t, roundDueAt.Equal(*bitcoin.LastSentRoundDueAt))
	assert.Equal(t, "long", bitcoin.AutoOrderPositionDirection)
	assert.True(t, decimal.RequireFromString("0.01").Equal(bitcoin.AutoOrderPositionQuantity))
	assert.Equal(t, "gt-ao-1-sl", bitcoin.AutoOrderStopLossClientID)
	solana, _ := rewrittenBot.WatchedSymbol("SOLUSDT")
	assert.Empty(t, solana.LastSentSignal)
	assert.Empty(t, solana.AutoOrderPositionDirection)

	// A symbol added back after being dropped starts blank.
	rewritten.WatchedSymbols = []entities.StrategyBotWatchedSymbol{{Symbol: "BTCUSDT"}, {Symbol: "ETHUSDT"}}
	readdedBot, readdError := repository.Save(t.Context(), rewritten)
	require.NoError(t, readdError)
	ethereum, _ := readdedBot.WatchedSymbol("ETHUSDT")
	assert.False(t, ethereum.Conflicting)
	assert.Empty(t, ethereum.HaltReason)
}

func TestStrategyBotRepositoryWritesEachSymbolsRoundStateToThatSymbolAlone(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	repository := persistence.NewStrategyBotRepository(database)
	savedBot := aBotWatching(t, database, "多標的", "BTCUSDT", "ETHUSDT")
	roundDueAt := time.Date(2026, 10, 20, 6, 0, 0, 0, time.UTC)
	for index := range savedBot.WatchedSymbols {
		savedBot.WatchedSymbols[index].LastSentSignal = string(vo.SignalBuy)
		savedBot.WatchedSymbols[index].LastSentRoundDueAt = &roundDueAt
	}
	require.NoError(t, repository.UpdateRunState(t.Context(), savedBot))
	require.NoError(t, repository.UpdateAutoOrderPosition(t.Context(), savedBot.ID, "ETHUSDT", vo.AutoOrderPositionVo{
		Direction: vo.TargetPositionShort, Quantity: decimal.RequireFromString("0.5"),
	}))

	require.NoError(t, repository.ForgetSentSignal(t.Context(), savedBot.ID, "ETHUSDT", roundDueAt))

	readBot, findError := repository.FindOne(t.Context(), savedBot.ID)
	require.NoError(t, findError)
	bitcoin, _ := readBot.WatchedSymbol("BTCUSDT")
	ethereum, _ := readBot.WatchedSymbol("ETHUSDT")
	assert.Equal(t, string(vo.SignalBuy), bitcoin.LastSentSignal)
	assert.Empty(t, bitcoin.AutoOrderPositionDirection)
	assert.Empty(t, ethereum.LastSentSignal)
	assert.Nil(t, ethereum.LastSentRoundDueAt)
	assert.Equal(t, "short", ethereum.AutoOrderPositionDirection)
}

func TestStrategyBotRunRecordRepositoryRemembersFiftyRoundsForEachSymbol(t *testing.T) {
	database := newStrategyBotTestDatabase(t)
	botID := aBotWatching(t, database, "多標的", "BTCUSDT", "ETHUSDT").ID
	repository := persistence.NewStrategyBotRunRecordRepository(database)

	for round := range 55 {
		requireAppended(t)(repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
			StrategyBotID: botID, Symbol: "BTCUSDT", RanAt: runRecordRanAt, Result: "hold"}))
		if round < 3 {
			requireAppended(t)(repository.Append(t.Context(), dto.StrategyBotRunRecordWriteDto{
				StrategyBotID: botID, Symbol: "ETHUSDT", RanAt: runRecordRanAt, Result: "buy"}))
		}
	}

	bitcoinHistory, findError := repository.FindLatestByBot(t.Context(), botID, "BTCUSDT")
	require.NoError(t, findError)
	ethereumHistory, findError := repository.FindLatestByBot(t.Context(), botID, "ETHUSDT")
	require.NoError(t, findError)
	wholeHistory, findError := repository.FindLatestByBot(t.Context(), botID, "")
	require.NoError(t, findError)

	assert.Len(t, bitcoinHistory, 50)
	require.Len(t, ethereumHistory, 3)
	assert.Len(t, wholeHistory, 53)
	// Numbers run across the bot's symbols and never repeat: the first round numbered BTC 1 and ETH 2.
	assert.Equal(t, 2, ethereumHistory[2].RunNumber)
	assert.Equal(t, "ETHUSDT", ethereumHistory[0].Symbol)
	assert.Equal(t, 58, wholeHistory[0].RunNumber)
	for index := 1; index < len(wholeHistory); index++ {
		assert.Greater(t, wholeHistory[index-1].RunNumber, wholeHistory[index].RunNumber)
	}
}

func TestContractAutoOrderRepositoryRunsEachSymbolsOrdersInTurnWithoutWaitingOnOthers(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	ethereumOrderOf := func(roundDueAt time.Time, runNumber int) entities.ContractAutoOrder {
		order := testBed.anOrderOf(testBed.botID, roundDueAt, runNumber)
		order.Symbol = "ETHUSDT"

		return order
	}
	for _, order := range []entities.ContractAutoOrder{
		testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 51),
		ethereumOrderOf(autoOrderQueuedAt, 52),
		testBed.anOrderOf(testBed.botID, autoOrderQueuedAt.Add(time.Minute), 53),
		ethereumOrderOf(autoOrderQueuedAt.Add(time.Minute), 54),
	} {
		require.NoError(t, testBed.repository.Enqueue(t.Context(), order))
	}
	stored := testBed.storedOrders(t)
	require.Len(t, stored, 4)

	candidates, findError := testBed.repository.FindDispatchCandidates(t.Context(), 10)
	require.NoError(t, findError)
	runNumbers := []int{}
	for _, candidate := range candidates {
		runNumbers = append(runNumbers, candidate.RunNumber)
	}
	assert.ElementsMatch(t, []int{51, 52}, runNumbers, "each symbol's oldest order goes ahead at once")

	bitcoinHasLater, laterError := testBed.repository.ExistsUnsettledAfter(t.Context(), testBed.botID, "BTCUSDT", stored[1].ID)
	require.NoError(t, laterError)
	assert.True(t, bitcoinHasLater, "a newer BTC order is queued after the first ETH one")
	ethereumHasLater, laterError := testBed.repository.ExistsUnsettledAfter(t.Context(), testBed.botID, "ETHUSDT", stored[3].ID)
	require.NoError(t, laterError)
	assert.False(t, ethereumHasLater, "another symbol's newer order is no reason to withdraw this one")

	hasUnsettled, unsettledError := testBed.repository.ExistsUnsettled(t.Context(), testBed.botID, "ETHUSDT")
	require.NoError(t, unsettledError)
	assert.True(t, hasUnsettled)
	hasUnsettled, unsettledError = testBed.repository.ExistsUnsettled(t.Context(), testBed.botID, "SOLUSDT")
	require.NoError(t, unsettledError)
	assert.False(t, hasUnsettled)
}

func TestContractAutoOrderRepositoryCountsOnlyUnsettledOrdersAsNotDone(t *testing.T) {
	testBed := newContractAutoOrderTestBed(t)
	settled := testBed.anOrderOf(testBed.botID, autoOrderQueuedAt, 51)
	settled.Status = string(vo.ContractAutoOrderSettled)
	require.NoError(t, testBed.repository.Enqueue(t.Context(), settled))

	hasUnsettled, unsettledError := testBed.repository.ExistsUnsettled(t.Context(), testBed.botID, "BTCUSDT")

	require.NoError(t, unsettledError)
	assert.False(t, hasUnsettled)
}
