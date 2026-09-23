package persistence_test

import (
	"encoding/json"
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
		IndexOpen:           storedFigure("102"),
		IndexHigh:           storedFigure("122"),
		IndexLow:            storedFigure("92"),
		IndexClose:          storedFigure("112"),
		PremiumIndexOpen:    storedFigure("-0.0001"),
		PremiumIndexHigh:    storedFigure("0.0002"),
		PremiumIndexLow:     storedFigure("-0.0003"),
		PremiumIndexClose:   storedFigure("0.0001"),
	}
}

func storedFigure(value string) decimal.NullDecimal {
	return decimal.NewNullDecimal(decimal.RequireFromString(value))
}

// candleStoredBeforeTheLaterLines is a contract K candle as it was stored before the
// index price and premium index existed: every other figure, and neither of those.
func candleStoredBeforeTheLaterLines(symbol string, openTime time.Time, closePrice string) entities.KCandleContract {
	return withoutPremiumIndex(withoutIndexPrice(contractCandleAt(symbol, openTime, closePrice)))
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

func withoutIndexPrice(candle entities.KCandleContract) entities.KCandleContract {
	candle.IndexOpen, candle.IndexHigh = decimal.NullDecimal{}, decimal.NullDecimal{}
	candle.IndexLow, candle.IndexClose = decimal.NullDecimal{}, decimal.NullDecimal{}

	return candle
}

func withoutPremiumIndex(candle entities.KCandleContract) entities.KCandleContract {
	candle.PremiumIndexOpen, candle.PremiumIndexHigh = decimal.NullDecimal{}, decimal.NullDecimal{}
	candle.PremiumIndexLow, candle.PremiumIndexClose = decimal.NullDecimal{}, decimal.NullDecimal{}

	return candle
}

func TestKCandleContractRepositoryCountsOnlyCandlesCarryingBothLaterLines(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)
	for _, candle := range []entities.KCandleContract{
		contractCandleAt("BTCUSDT", at(9, 0), "100"),
		candleStoredBeforeTheLaterLines("BTCUSDT", at(9, 1), "100"),
		contractCandleAt("BTCUSDT", at(9, 2), "100"),
		withoutIndexPrice(contractCandleAt("BTCUSDT", at(9, 3), "100")),
		withoutPremiumIndex(contractCandleAt("BTCUSDT", at(9, 4), "100")),
	} {
		_, saveError := contractRepository.Save(t.Context(), candle)
		require.NoError(t, saveError)
	}

	heldCount, countError := contractRepository.CountInRange(
		t.Context(), "BTCUSDT", at(9, 0), at(9, 4))

	require.NoError(t, countError)
	assert.Equal(t, 2, heldCount)
}

func TestKCandleContractRepositorySaveAllIfAbsentFillsInOnlyTheLinesAnOldCandleLacks(t *testing.T) {
	testCases := []struct {
		name       string
		heldCandle entities.KCandleContract
	}{
		{name: "兩組都缺", heldCandle: candleStoredBeforeTheLaterLines("BTCUSDT", at(9, 0), "100")},
		{name: "只缺指數價格", heldCandle: withoutIndexPrice(contractCandleAt("BTCUSDT", at(9, 0), "100"))},
		{name: "只缺溢價指數", heldCandle: withoutPremiumIndex(contractCandleAt("BTCUSDT", at(9, 0), "100"))},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			database := newTestDatabase(t)
			contractRepository := persistence.NewKCandleContractRepository(database)
			_, heldError := contractRepository.Save(t.Context(), testCase.heldCandle)
			require.NoError(t, heldError)
			refetched := contractCandleAt("BTCUSDT", at(9, 0), "999")
			refetched.MarkClose = decimal.RequireFromString("888")
			refetched.TradeCount = 1

			storedCount, saveError := contractRepository.SaveAllIfAbsent(
				t.Context(), []entities.KCandleContract{refetched})

			require.NoError(t, saveError)
			assert.Equal(t, 1, storedCount)
			held, readError := contractRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
			require.NoError(t, readError)
			assert.True(t, decimal.RequireFromString("100").Equal(held.Close))
			assert.True(t, decimal.RequireFromString("111").Equal(held.MarkClose))
			assert.Equal(t, int64(42), held.TradeCount)
			require.True(t, held.IndexClose.Valid)
			assert.True(t, decimal.RequireFromString("102").Equal(held.IndexOpen.Decimal))
			assert.True(t, decimal.RequireFromString("122").Equal(held.IndexHigh.Decimal))
			assert.True(t, decimal.RequireFromString("92").Equal(held.IndexLow.Decimal))
			assert.True(t, decimal.RequireFromString("112").Equal(held.IndexClose.Decimal))
			require.True(t, held.PremiumIndexClose.Valid)
			assert.True(t, decimal.RequireFromString("-0.0001").Equal(held.PremiumIndexOpen.Decimal))
			assert.True(t, decimal.RequireFromString("0.0002").Equal(held.PremiumIndexHigh.Decimal))
			assert.True(t, decimal.RequireFromString("-0.0003").Equal(held.PremiumIndexLow.Decimal))
			assert.True(t, decimal.RequireFromString("0.0001").Equal(held.PremiumIndexClose.Decimal))
		})
	}
}

func TestKCandleContractRepositorySaveAllIfAbsentLeavesACompleteCandleExactlyAsItWas(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)
	_, heldError := contractRepository.Save(t.Context(), contractCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, heldError)
	refetched := contractCandleAt("BTCUSDT", at(9, 0), "999")
	refetched.IndexClose = storedFigure("5")
	refetched.PremiumIndexClose = storedFigure("0.5")

	storedCount, saveError := contractRepository.SaveAllIfAbsent(
		t.Context(), []entities.KCandleContract{refetched})

	require.NoError(t, saveError)
	assert.Equal(t, 0, storedCount)
	held, readError := contractRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
	require.NoError(t, readError)
	assert.True(t, decimal.RequireFromString("100").Equal(held.Close))
	assert.True(t, decimal.RequireFromString("112").Equal(held.IndexClose.Decimal))
	assert.True(t, decimal.RequireFromString("0.0001").Equal(held.PremiumIndexClose.Decimal))
}

func TestKCandleContractRepositoryHandsAnOldCandleOutWithoutTheLaterLines(t *testing.T) {
	database := newTestDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)
	_, heldError := contractRepository.Save(
		t.Context(), candleStoredBeforeTheLaterLines("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, heldError)
	query, queryError := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
		Symbol: "BTCUSDT", StartTime: at(9, 0), EndTime: at(9, 0),
	})
	require.NoError(t, queryError)

	heldCandles, readError := contractRepository.FindInRange(t.Context(), query, 10)

	require.NoError(t, readError)
	require.Len(t, heldCandles, 1)
	handedOut := heldCandles[0].ToDto()
	assert.True(t, decimal.RequireFromString("111").Equal(handedOut.MarkClose))
	assert.False(t, handedOut.IndexOpen.Valid)
	assert.False(t, handedOut.IndexClose.Valid)
	assert.False(t, handedOut.PremiumIndexOpen.Valid)
	assert.False(t, handedOut.PremiumIndexClose.Valid)
	serialized, marshalError := json.Marshal(handedOut)
	require.NoError(t, marshalError)
	assert.Contains(t, string(serialized), `"indexClose":null`)
	assert.Contains(t, string(serialized), `"premiumIndexClose":null`)
}

func TestKCandleContractRepositoryReplacesAnOldCandleInsideTheRecentMinutesWithTheCompleteOne(t *testing.T) {
	// The every-minute round re-fetches its recent minutes and replaces what it
	// finds; an old candle among them becomes the complete one it was fetched as.
	database := newTestDatabase(t)
	contractRepository := persistence.NewKCandleContractRepository(database)
	_, heldError := contractRepository.Save(
		t.Context(), candleStoredBeforeTheLaterLines("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, heldError)

	_, saveError := contractRepository.Save(t.Context(), contractCandleAt("BTCUSDT", at(9, 0), "105"))

	require.NoError(t, saveError)
	held, readError := contractRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
	require.NoError(t, readError)
	assert.True(t, decimal.RequireFromString("105").Equal(held.Close))
	require.True(t, held.IndexClose.Valid)
	assert.True(t, decimal.RequireFromString("112").Equal(held.IndexClose.Decimal))
	require.True(t, held.PremiumIndexClose.Valid)
	assert.True(t, decimal.RequireFromString("0.0001").Equal(held.PremiumIndexClose.Decimal))
}
