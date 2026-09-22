package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContractTradingSymbolRepositoryHoldsTheSameNameAsTheSpotList(t *testing.T) {
	database := newTestDatabase(t)
	spotRepository := persistence.NewTradingSymbolRepository(database)
	contractRepository := persistence.NewContractTradingSymbolRepository(database)
	require.NoError(t, spotRepository.Save(t.Context(), entities.TradingSymbol{
		Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
	}))

	saveError := contractRepository.Save(t.Context(), entities.ContractTradingSymbol{
		Symbol: "BTCUSDT", IsWatched: true,
	})

	require.NoError(t, saveError)
	spotSymbol, spotFound, spotError := spotRepository.FindBySymbol(t.Context(), "BTCUSDT")
	require.NoError(t, spotError)
	require.True(t, spotFound)
	assert.True(t, spotSymbol.IsWatched)
	contractSymbol, contractFound, contractError := contractRepository.FindBySymbol(t.Context(), "BTCUSDT")
	require.NoError(t, contractError)
	require.True(t, contractFound)
	assert.True(t, contractSymbol.IsWatched)
}

func TestContractTradingSymbolRepositoryStoresStoppingToFollow(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewContractTradingSymbolRepository(database)
	require.NoError(t, contractRepository.Save(t.Context(), entities.ContractTradingSymbol{
		Symbol: "BTCUSDT", IsWatched: true,
	}))

	saveError := contractRepository.Save(t.Context(), entities.ContractTradingSymbol{
		Symbol: "BTCUSDT", IsWatched: false,
	})

	require.NoError(t, saveError)
	stored, found, readError := contractRepository.FindBySymbol(t.Context(), "BTCUSDT")
	require.NoError(t, readError)
	require.True(t, found)
	assert.False(t, stored.IsWatched)
	watchedSymbols, watchedError := contractRepository.FindWatched(t.Context())
	require.NoError(t, watchedError)
	assert.Empty(t, watchedSymbols)
}

func TestContractTradingSymbolRepositoryReadsRegisteredAndWatchedByName(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewContractTradingSymbolRepository(database)
	require.NoError(t, contractRepository.Save(t.Context(), entities.ContractTradingSymbol{
		Symbol: "ETHUSDT", IsWatched: true,
	}))
	require.NoError(t, contractRepository.Save(t.Context(), entities.ContractTradingSymbol{
		Symbol: "BTCUSDT", IsWatched: true,
	}))
	require.NoError(t, contractRepository.Save(t.Context(), entities.ContractTradingSymbol{
		Symbol: "1000PEPEUSDT", IsWatched: false,
	}))

	allSymbols, allError := contractRepository.FindAll(t.Context())
	watchedSymbols, watchedError := contractRepository.FindWatched(t.Context())

	require.NoError(t, allError)
	require.NoError(t, watchedError)
	assert.Equal(t, []string{"1000PEPEUSDT", "BTCUSDT", "ETHUSDT"}, symbolNamesOf(allSymbols))
	assert.Equal(t, []string{"BTCUSDT", "ETHUSDT"}, symbolNamesOf(watchedSymbols))
}

func TestContractTradingSymbolRepositoryAnswersNotFoundForAnUnknownName(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewContractTradingSymbolRepository(database)

	_, found, readError := contractRepository.FindBySymbol(t.Context(), "BTCUSDT")

	require.NoError(t, readError)
	assert.False(t, found)
}

func TestContractTradingSymbolRepositorySaysSoWhenStorageIsUnreachable(t *testing.T) {
	database := closedDatabase(t)
	contractRepository := persistence.NewContractTradingSymbolRepository(database)

	_, allError := contractRepository.FindAll(t.Context())
	_, watchedError := contractRepository.FindWatched(t.Context())
	_, _, findError := contractRepository.FindBySymbol(t.Context(), "BTCUSDT")
	saveError := contractRepository.Save(t.Context(), entities.ContractTradingSymbol{Symbol: "BTCUSDT"})

	for _, storageError := range []error{allError, watchedError, findError, saveError} {
		assert.Error(t, storageError)
	}
}

func symbolNamesOf(contractTradingSymbols []entities.ContractTradingSymbol) []string {
	names := make([]string, 0, len(contractTradingSymbols))
	for _, contractTradingSymbol := range contractTradingSymbols {
		names = append(names, contractTradingSymbol.Symbol)
	}

	return names
}

func runningContractSyncRun(symbol string) entities.KCandleContractHistorySyncRun {
	return entities.KCandleContractHistorySyncRun{
		Symbol:       symbol,
		LookbackDays: 30,
		Status:       string(vo.KCandleHistorySyncRunning),
		TotalChunks:  30,
		StartedAt:    time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC),
	}
}

func TestContractHistorySyncRunRepositoryRefusesASecondRunOnTheSameSymbol(t *testing.T) {
	database := newTestDatabase(t)
	runRepository := persistence.NewKCandleContractHistorySyncRunRepository(database)
	_, firstError := runRepository.Save(t.Context(), runningContractSyncRun("BTCUSDT"))
	require.NoError(t, firstError)

	_, secondError := runRepository.Save(t.Context(), runningContractSyncRun("BTCUSDT"))
	_, otherSymbolError := runRepository.Save(t.Context(), runningContractSyncRun("ETHUSDT"))

	assert.ErrorIs(t, secondError, domains.ErrKCandleHistorySyncInProgress)
	assert.NoError(t, otherSymbolError)
}

func TestContractHistorySyncRunRepositoryIsUnaffectedByASpotRunOnTheSameSymbol(t *testing.T) {
	database := newTestDatabase(t)
	spotRunRepository := persistence.NewKCandleHistorySyncRunRepository(database)
	contractRunRepository := persistence.NewKCandleContractHistorySyncRunRepository(database)
	_, spotError := spotRunRepository.Save(t.Context(), entities.KCandleHistorySyncRun{
		Symbol:       "BTCUSDT",
		LookbackDays: 30,
		Status:       string(vo.KCandleHistorySyncRunning),
		StartedAt:    time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC),
	})
	require.NoError(t, spotError)

	contractRun, contractError := contractRunRepository.Save(
		t.Context(), runningContractSyncRun("BTCUSDT"))

	require.NoError(t, contractError)
	assert.Positive(t, contractRun.ID)
}

func TestContractHistorySyncRunRepositoryReadsARunBackAndSweepsAnInterruptedOne(t *testing.T) {
	database := newTestDatabase(t)
	runRepository := persistence.NewKCandleContractHistorySyncRunRepository(database)
	startedRun, startError := runRepository.Save(t.Context(), runningContractSyncRun("BTCUSDT"))
	require.NoError(t, startError)
	sweptAt := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	sweptCount, sweepError := runRepository.FailAllRunning(
		t.Context(), "interrupted by restart", sweptAt)

	require.NoError(t, sweepError)
	assert.Equal(t, 1, sweptCount)
	storedRun, found, readError := runRepository.FindOne(t.Context(), startedRun.ID)
	require.NoError(t, readError)
	require.True(t, found)
	assert.Equal(t, string(vo.KCandleHistorySyncFailed), storedRun.Status)
	assert.Equal(t, "interrupted by restart", storedRun.FailureReason)
	require.NotNil(t, storedRun.FinishedAt)
	assert.Equal(t, sweptAt, storedRun.FinishedAt.UTC())
}

func TestContractHistorySyncRunRepositoryAnswersNotFoundForAnUnknownRun(t *testing.T) {
	database := newTestDatabase(t)
	runRepository := persistence.NewKCandleContractHistorySyncRunRepository(database)

	_, found, readError := runRepository.FindOne(t.Context(), 4242)

	require.NoError(t, readError)
	assert.False(t, found)
}

func TestContractHistorySyncRunRepositorySaysSoWhenStorageIsUnreachable(t *testing.T) {
	database := closedDatabase(t)
	runRepository := persistence.NewKCandleContractHistorySyncRunRepository(database)

	_, saveError := runRepository.Save(t.Context(), runningContractSyncRun("BTCUSDT"))
	_, _, findError := runRepository.FindOne(t.Context(), 1)
	_, sweepError := runRepository.FailAllRunning(t.Context(), "boom", time.Now().UTC())

	for _, storageError := range []error{saveError, findError, sweepError} {
		assert.Error(t, storageError)
	}
}
