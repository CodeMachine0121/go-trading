package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
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

func specifiedContract(symbol string, isWatched bool, tickSize string, confirmedAt time.Time) entities.ContractTradingSymbol {
	eightHours := 8

	return entities.ContractTradingSymbol{
		Symbol:                 symbol,
		IsWatched:              isWatched,
		TickSize:               decimal.NewNullDecimal(decimal.RequireFromString(tickSize)),
		QuantityStep:           decimal.NewNullDecimal(decimal.RequireFromString("0.001")),
		MinimumQuantity:        decimal.NewNullDecimal(decimal.RequireFromString("0.001")),
		MinimumNotional:        decimal.NewNullDecimal(decimal.RequireFromString("50")),
		MaintenanceMarginRate:  decimal.NewNullDecimal(decimal.RequireFromString("0.025")),
		LiquidationFeeRate:     decimal.NewNullDecimal(decimal.RequireFromString("0.0125")),
		FundingIntervalHours:   &eightHours,
		SpecificationUpdatedAt: &confirmedAt,
	}
}

func TestContractTradingSymbolRepositorySavesTheSpecificationOnlyWhenOneIsCarried(t *testing.T) {
	database := newTestDatabase(t)
	symbolRepository := persistence.NewContractTradingSymbolRepository(database)
	require.NoError(t, symbolRepository.Save(t.Context(), specifiedContract("BTCUSDT", true, "0.1", at(8, 0))))

	require.NoError(t, symbolRepository.Save(t.Context(),
		entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: false}))

	held, found, findError := symbolRepository.FindBySymbol(t.Context(), "BTCUSDT")
	require.NoError(t, findError)
	require.True(t, found)
	assert.False(t, held.IsWatched)
	assert.True(t, decimal.RequireFromString("0.1").Equal(held.TickSize.Decimal))
	require.NotNil(t, held.SpecificationUpdatedAt)
	assert.Equal(t, at(8, 0), held.SpecificationUpdatedAt.UTC())

	require.NoError(t, symbolRepository.Save(t.Context(), specifiedContract("BTCUSDT", true, "0.01", at(9, 0))))

	replaced, _, replacedError := symbolRepository.FindBySymbol(t.Context(), "BTCUSDT")
	require.NoError(t, replacedError)
	assert.True(t, replaced.IsWatched)
	assert.True(t, decimal.RequireFromString("0.01").Equal(replaced.TickSize.Decimal))
	assert.Equal(t, at(9, 0), replaced.SpecificationUpdatedAt.UTC())
}

func TestContractTradingSymbolRepositoryRefreshWritesTheSpecificationAlone(t *testing.T) {
	database := newTestDatabase(t)
	symbolRepository := persistence.NewContractTradingSymbolRepository(database)
	require.NoError(t, symbolRepository.Save(t.Context(), entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}))
	require.NoError(t, symbolRepository.Save(t.Context(), entities.ContractTradingSymbol{Symbol: "ETHUSDT", IsWatched: false}))

	refreshError := symbolRepository.SaveTradingSpecifications(t.Context(), []entities.ContractTradingSymbol{
		specifiedContract("BTCUSDT", false, "0.1", at(8, 0)),
		specifiedContract("NOTREGISTEREDUSDT", true, "0.1", at(8, 0)),
	})

	require.NoError(t, refreshError)
	bitcoin, _, _ := symbolRepository.FindBySymbol(t.Context(), "BTCUSDT")
	assert.True(t, bitcoin.IsWatched)
	assert.True(t, decimal.RequireFromString("0.1").Equal(bitcoin.TickSize.Decimal))
	require.NotNil(t, bitcoin.FundingIntervalHours)
	assert.Equal(t, 8, *bitcoin.FundingIntervalHours)
	ether, _, _ := symbolRepository.FindBySymbol(t.Context(), "ETHUSDT")
	assert.False(t, ether.TickSize.Valid)
	assert.Nil(t, ether.SpecificationUpdatedAt)
	_, created, _ := symbolRepository.FindBySymbol(t.Context(), "NOTREGISTEREDUSDT")
	assert.False(t, created)
}

func TestContractTradingSymbolRepositoryRefreshSaysSoWhenStorageIsUnreachable(t *testing.T) {
	database := newTestDatabase(t)
	symbolRepository := persistence.NewContractTradingSymbolRepository(database)
	connection, connectionError := database.DB()
	require.NoError(t, connectionError)
	require.NoError(t, connection.Close())

	refreshError := symbolRepository.SaveTradingSpecifications(t.Context(),
		[]entities.ContractTradingSymbol{specifiedContract("BTCUSDT", true, "0.1", at(8, 0))})

	assert.ErrorContains(t, refreshError, "save contract trading specifications")
}

func TestContractTradingSymbolRepositoryRefreshRecordsNothingWhenOneContractCannotBeWritten(t *testing.T) {
	database := newTestDatabase(t)
	symbolRepository := persistence.NewContractTradingSymbolRepository(database)
	require.NoError(t, symbolRepository.Save(t.Context(), entities.ContractTradingSymbol{Symbol: "BTCUSDT"}))

	// PostgreSQL refuses a NUL inside text, so the second row fails mid-transaction.
	refreshError := symbolRepository.SaveTradingSpecifications(t.Context(), []entities.ContractTradingSymbol{
		specifiedContract("BTCUSDT", false, "0.1", at(8, 0)),
		specifiedContract("BAD\x00USDT", false, "0.1", at(8, 0)),
	})

	assert.ErrorContains(t, refreshError, "save contract trading specifications")
	bitcoin, _, _ := symbolRepository.FindBySymbol(t.Context(), "BTCUSDT")
	assert.False(t, bitcoin.TickSize.Valid)
}
