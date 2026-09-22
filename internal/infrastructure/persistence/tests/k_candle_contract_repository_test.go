package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func contractCandleAt(symbol string, openTime time.Time, closePrice string) entities.KCandleContract {
	return entities.KCandleContract{
		Symbol:              symbol,
		OpenTime:            openTime,
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("120"),
		Low:                 decimal.RequireFromString("90"),
		Close:               decimal.RequireFromString(closePrice),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.RequireFromString("1200"),
		TakerBuyBaseVolume:  decimal.RequireFromString("5"),
		TakerBuyQuoteVolume: decimal.RequireFromString("600"),
		TradeCount:          42,
		MarkOpen:            decimal.RequireFromString("101"),
		MarkHigh:            decimal.RequireFromString("121"),
		MarkLow:             decimal.RequireFromString("91"),
		MarkClose:           decimal.RequireFromString("111"),
	}
}

func TestKCandleContractRepositoryKeepsContractAndSpotCandlesApart(t *testing.T) {
	database := newTestDatabase(t)
	spotRepository := persistence.NewKCandleRepository(database)
	contractRepository := persistence.NewKCandleContractRepository(database)
	_, spotError := spotRepository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, spotError)

	_, contractError := contractRepository.Save(
		t.Context(), contractCandleAt("BTCUSDT", at(9, 0), "101"))

	require.NoError(t, contractError)
	storedSpot, readSpotError := spotRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
	require.NoError(t, readSpotError)
	assert.True(t, decimal.RequireFromString("100").Equal(storedSpot.Close))
	storedContract, readContractError := contractRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
	require.NoError(t, readContractError)
	assert.True(t, decimal.RequireFromString("101").Equal(storedContract.Close))
	assert.True(t, decimal.RequireFromString("111").Equal(storedContract.MarkClose))
	assert.Equal(t, int64(42), storedContract.TradeCount)
}

func TestKCandleContractRepositoryDeleteLeavesTheSpotCandleAlone(t *testing.T) {
	database := newTestDatabase(t)
	spotRepository := persistence.NewKCandleRepository(database)
	contractRepository := persistence.NewKCandleContractRepository(database)
	_, spotError := spotRepository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, spotError)
	_, contractError := contractRepository.Save(
		t.Context(), contractCandleAt("BTCUSDT", at(9, 0), "101"))
	require.NoError(t, contractError)

	deleteError := contractRepository.Delete(t.Context(), "BTCUSDT", at(9, 0))

	require.NoError(t, deleteError)
	_, readContractError := contractRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
	assert.ErrorIs(t, readContractError, domains.ErrKCandleContractNotFound)
	storedSpot, readSpotError := spotRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
	require.NoError(t, readSpotError)
	assert.True(t, decimal.RequireFromString("100").Equal(storedSpot.Close))
}

func TestKCandleContractRepositoryOverwritesTheSameSymbolAndOpenTime(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)
	_, firstError := contractRepository.Save(
		t.Context(), contractCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, firstError)

	_, secondError := contractRepository.Save(
		t.Context(), contractCandleAt("BTCUSDT", at(9, 0), "120"))

	require.NoError(t, secondError)
	heldCount, countError := contractRepository.CountInRange(
		t.Context(), "BTCUSDT", at(9, 0), at(9, 0))
	require.NoError(t, countError)
	assert.Equal(t, 1, heldCount)
	stored, readError := contractRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
	require.NoError(t, readError)
	assert.True(t, decimal.RequireFromString("120").Equal(stored.Close))
}

func TestKCandleContractRepositoryTreatsLookAlikeSymbolsAsUnrelated(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)
	_, shortNameError := contractRepository.Save(
		t.Context(), contractCandleAt("SHIBUSDT", at(9, 0), "0.00001"))
	require.NoError(t, shortNameError)
	_, longNameError := contractRepository.Save(
		t.Context(), contractCandleAt("1000SHIBUSDT", at(9, 0), "0.01"))
	require.NoError(t, longNameError)

	storedSymbols, readError := contractRepository.FindDistinctSymbols(t.Context())

	require.NoError(t, readError)
	assert.Equal(t, []string{"1000SHIBUSDT", "SHIBUSDT"}, storedSymbols)
	shortNameCandle, shortReadError := contractRepository.FindOne(t.Context(), "SHIBUSDT", at(9, 0))
	require.NoError(t, shortReadError)
	assert.True(t, decimal.RequireFromString("0.00001").Equal(shortNameCandle.Close))
}

func TestKCandleContractRepositorySaveAllIfAbsentLeavesHeldCandlesUntouched(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)
	_, heldError := contractRepository.Save(
		t.Context(), contractCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, heldError)

	storedCount, saveError := contractRepository.SaveAllIfAbsent(t.Context(), []entities.KCandleContract{
		contractCandleAt("BTCUSDT", at(9, 0), "999"),
		contractCandleAt("BTCUSDT", at(9, 1), "101"),
	})

	require.NoError(t, saveError)
	assert.Equal(t, 1, storedCount)
	held, readError := contractRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
	require.NoError(t, readError)
	assert.True(t, decimal.RequireFromString("100").Equal(held.Close))
}

func TestKCandleContractRepositorySaveAllIfAbsentAcceptsAnEmptyBatch(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)

	storedCount, saveError := contractRepository.SaveAllIfAbsent(
		t.Context(), []entities.KCandleContract{})

	require.NoError(t, saveError)
	assert.Equal(t, 0, storedCount)
}

func TestKCandleContractRepositoryReadsARangeEarliestFirst(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)
	_, saveError := contractRepository.SaveAllIfAbsent(t.Context(), []entities.KCandleContract{
		contractCandleAt("BTCUSDT", at(9, 2), "102"),
		contractCandleAt("BTCUSDT", at(9, 0), "100"),
		contractCandleAt("BTCUSDT", at(9, 1), "101"),
	})
	require.NoError(t, saveError)
	query, queryError := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
		Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(9, 2)})
	require.NoError(t, queryError)

	storedCandles, readError := contractRepository.FindInRange(t.Context(), query, 1000)

	require.NoError(t, readError)
	require.Len(t, storedCandles, 3)
	assert.Equal(t, at(9, 0), storedCandles[0].OpenTime.UTC())
	assert.Equal(t, at(9, 2), storedCandles[2].OpenTime.UTC())
}

func TestKCandleContractRepositoryReadsTheLatestCandlesNewestFirst(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)
	_, saveError := contractRepository.SaveAllIfAbsent(t.Context(), []entities.KCandleContract{
		contractCandleAt("BTCUSDT", at(9, 0), "100"),
		contractCandleAt("BTCUSDT", at(9, 1), "101"),
	})
	require.NoError(t, saveError)

	latestCandles, readError := contractRepository.FindLatest(t.Context(), "BTCUSDT", 1)

	require.NoError(t, readError)
	require.Len(t, latestCandles, 1)
	assert.Equal(t, at(9, 1), latestCandles[0].OpenTime.UTC())
}

func TestKCandleContractRepositoryUpdatesTheFiguresOfAHeldCandle(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)
	_, saveError := contractRepository.Save(t.Context(), contractCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, saveError)
	amendedCandle := contractCandleAt("BTCUSDT", at(9, 0), "120")
	amendedCandle.MarkClose = decimal.RequireFromString("121")
	amendedCandle.TradeCount = 7

	_, updateError := contractRepository.Update(t.Context(), amendedCandle)

	require.NoError(t, updateError)
	stored, readError := contractRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
	require.NoError(t, readError)
	assert.True(t, decimal.RequireFromString("120").Equal(stored.Close))
	assert.True(t, decimal.RequireFromString("121").Equal(stored.MarkClose))
	assert.Equal(t, int64(7), stored.TradeCount)
}

func TestKCandleContractRepositoryReportsNotFound(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)

	_, readError := contractRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
	_, updateError := contractRepository.Update(t.Context(), contractCandleAt("BTCUSDT", at(9, 0), "100"))
	deleteError := contractRepository.Delete(t.Context(), "BTCUSDT", at(9, 0))

	assert.ErrorIs(t, readError, domains.ErrKCandleContractNotFound)
	assert.ErrorIs(t, updateError, domains.ErrKCandleContractNotFound)
	assert.ErrorIs(t, deleteError, domains.ErrKCandleContractNotFound)
}

func TestKCandleContractRepositorySaysSoWhenStorageIsUnreachable(t *testing.T) {
	database := closedDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)
	query, queryError := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
		Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(9, 2)})
	require.NoError(t, queryError)

	_, saveError := contractRepository.Save(t.Context(), contractCandleAt("BTCUSDT", at(9, 0), "100"))
	_, batchError := contractRepository.SaveAllIfAbsent(
		t.Context(), []entities.KCandleContract{contractCandleAt("BTCUSDT", at(9, 0), "100")})
	_, countError := contractRepository.CountInRange(t.Context(), "BTCUSDT", at(9, 0), at(9, 2))
	_, updateError := contractRepository.Update(t.Context(), contractCandleAt("BTCUSDT", at(9, 0), "100"))
	_, findOneError := contractRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
	_, rangeError := contractRepository.FindInRange(t.Context(), query, 1000)
	_, symbolsError := contractRepository.FindDistinctSymbols(t.Context())
	_, latestError := contractRepository.FindLatest(t.Context(), "BTCUSDT", 1)
	deleteError := contractRepository.Delete(t.Context(), "BTCUSDT", at(9, 0))

	for _, storageError := range []error{
		saveError, batchError, countError, updateError, findOneError,
		rangeError, symbolsError, latestError, deleteError,
	} {
		assert.Error(t, storageError)
	}
}
