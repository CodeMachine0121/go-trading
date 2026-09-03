package persistence_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// testDatabaseNameSuffix is what a database must be called before these tests will
// touch it. They empty the table on every run, so pointing them at the database the
// application actually uses destroys real data — and a developer who typed the wrong
// DSN finds that out only afterwards. Refusing anything not named for testing turns
// that mistake into a failed test instead of lost K candles.
const testDatabaseNameSuffix = "_test"

// newTestDatabase connects to the PostgreSQL instance named by TEST_POSTGRES_DSN and
// starts each test from an empty table. Without that variable the storage behaviors
// cannot be exercised, so the tests skip rather than pretend to pass.
func newTestDatabase(t *testing.T) *gorm.DB {
	dataSourceName := os.Getenv("TEST_POSTGRES_DSN")
	if dataSourceName == "" {
		t.Skip("TEST_POSTGRES_DSN is not set; skipping storage tests")
	}

	require.True(t, strings.HasSuffix(databaseNameIn(dataSourceName), testDatabaseNameSuffix),
		"TEST_POSTGRES_DSN 指向的資料庫必須以 %s 結尾——這些測試每次都會清空 KCandles，"+
			"指到應用程式在用的那一個會毀掉真實資料", testDatabaseNameSuffix)

	database, err := persistence.NewDatabase(dataSourceName)
	require.NoError(t, err)
	_, err = persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, err)
	clearedDatabase := database.Session(&gorm.Session{AllowGlobalUpdate: true})
	require.NoError(t, clearedDatabase.WithContext(t.Context()).Delete(&entities.KCandle{}).Error)
	require.NoError(t, clearedDatabase.WithContext(t.Context()).Delete(&entities.TradingSymbol{}).Error)
	require.NoError(t, clearedDatabase.WithContext(t.Context()).Delete(&entities.Strategy{}).Error)

	return database
}

// databaseNameIn reads the dbname out of a key=value DSN, answering with an empty
// name when the DSN does not carry one — which fails the guard above, as it should.
func databaseNameIn(dataSourceName string) string {
	for _, setting := range strings.Fields(dataSourceName) {
		name, found := strings.CutPrefix(setting, "dbname=")
		if found {
			return name
		}
	}

	return ""
}

// closedDatabase hands back a connection that is already shut, which is the only way
// to make the storage layer fail on demand. Every read and write must say so rather
// than quietly answering with nothing.
func closedDatabase(t *testing.T) *gorm.DB {
	database := newTestDatabase(t)
	connection, connectionError := database.DB()
	require.NoError(t, connectionError)
	require.NoError(t, connection.Close())

	return database
}

func at(hour int, minute int) time.Time {
	return time.Date(2026, 8, 29, hour, minute, 0, 0, time.UTC)
}

func kCandleAt(symbol string, openTime time.Time, closePrice string) entities.KCandle {
	return entities.KCandle{
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
	}
}

func queryFor(t *testing.T, symbol string, startTime time.Time, endTime time.Time) domains.KCandleQueryDomain {
	query, err := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
		Symbol: symbol, StartTime: startTime, EndTime: endTime,
	})
	require.NoError(t, err)
	return query
}

func TestSaveOverwritesTheCandleAlreadyHeldForTheSameSymbolAndOpenTime(t *testing.T) {
	kCandleRepository := persistence.NewKCandleRepository(newTestDatabase(t))
	_, err := kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, err)

	_, err = kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "120"))
	require.NoError(t, err)

	storedKCandles, err := kCandleRepository.FindInRange(t.Context(), queryFor(t, "BTCUSDT", at(9, 0), at(9, 0)), 10)
	assert.NoError(t, err)
	assert.Len(t, storedKCandles, 1)
	assert.True(t, decimal.RequireFromString("120").Equal(storedKCandles[0].Close))
}

func TestSaveKeepsCandlesOfDifferentSymbolsApart(t *testing.T) {
	kCandleRepository := persistence.NewKCandleRepository(newTestDatabase(t))
	_, err := kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, err)
	_, err = kCandleRepository.Save(t.Context(), kCandleAt("ETHUSDT", at(9, 0), "200"))
	require.NoError(t, err)

	storedKCandles, err := kCandleRepository.FindInRange(t.Context(), queryFor(t, "ETHUSDT", at(9, 0), at(9, 0)), 10)
	assert.NoError(t, err)
	assert.Len(t, storedKCandles, 1)
	assert.True(t, decimal.RequireFromString("200").Equal(storedKCandles[0].Close))
}

func TestFindInRange(t *testing.T) {
	kCandleRepository := persistence.NewKCandleRepository(newTestDatabase(t))
	for _, openTime := range []time.Time{at(9, 10), at(9, 0), at(9, 5)} {
		_, err := kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", openTime, "100"))
		require.NoError(t, err)
	}

	testCases := []struct {
		name              string
		startTime         time.Time
		endTime           time.Time
		limit             int
		expectedOpenTimes []time.Time
	}{
		{
			name:              "returns every candle in the range, earliest first",
			startTime:         at(9, 0),
			endTime:           at(9, 10),
			limit:             10,
			expectedOpenTimes: []time.Time{at(9, 0), at(9, 5), at(9, 10)},
		},
		{
			name:              "includes both ends of the range",
			startTime:         at(9, 5),
			endTime:           at(9, 5),
			limit:             10,
			expectedOpenTimes: []time.Time{at(9, 5)},
		},
		{
			name:              "accepts a range whose ends are off the five minute mark",
			startTime:         at(9, 1),
			endTime:           at(9, 9),
			limit:             10,
			expectedOpenTimes: []time.Time{at(9, 5)},
		},
		{
			name:              "returns nothing when the range holds no candle",
			startTime:         at(11, 0),
			endTime:           at(12, 0),
			limit:             10,
			expectedOpenTimes: []time.Time{},
		},
		{
			name:              "returns no more than the limit",
			startTime:         at(9, 0),
			endTime:           at(9, 10),
			limit:             2,
			expectedOpenTimes: []time.Time{at(9, 0), at(9, 5)},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			storedKCandles, err := kCandleRepository.FindInRange(t.Context(),
				queryFor(t, "BTCUSDT", testCase.startTime, testCase.endTime), testCase.limit)

			assert.NoError(t, err)
			assert.Len(t, storedKCandles, len(testCase.expectedOpenTimes))
			for index, expectedOpenTime := range testCase.expectedOpenTimes {
				assert.Equal(t, expectedOpenTime, storedKCandles[index].OpenTime.UTC())
			}
		})
	}
}

func TestFindOne(t *testing.T) {
	kCandleRepository := persistence.NewKCandleRepository(newTestDatabase(t))
	_, err := kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "110"))
	require.NoError(t, err)

	t.Run("returns the named candle", func(t *testing.T) {
		storedKCandle, err := kCandleRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))

		assert.NoError(t, err)
		assert.True(t, decimal.RequireFromString("110").Equal(storedKCandle.Close))
	})

	t.Run("reports not found when no candle carries that symbol and open time", func(t *testing.T) {
		_, err := kCandleRepository.FindOne(t.Context(), "BTCUSDT", at(9, 5))

		assert.ErrorIs(t, err, domains.ErrKCandleNotFound)
	})
}

func TestUpdate(t *testing.T) {
	kCandleRepository := persistence.NewKCandleRepository(newTestDatabase(t))
	_, err := kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, err)

	t.Run("replaces the figures of an existing candle", func(t *testing.T) {
		_, err := kCandleRepository.Update(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "120"))
		assert.NoError(t, err)

		storedKCandle, err := kCandleRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
		assert.NoError(t, err)
		assert.True(t, decimal.RequireFromString("120").Equal(storedKCandle.Close))
	})

	t.Run("stores a figure of zero rather than skipping it", func(t *testing.T) {
		zeroVolumeKCandle := kCandleAt("BTCUSDT", at(9, 0), "120")
		zeroVolumeKCandle.Volume = decimal.Zero

		_, err := kCandleRepository.Update(t.Context(), zeroVolumeKCandle)
		assert.NoError(t, err)

		storedKCandle, err := kCandleRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
		assert.NoError(t, err)
		assert.True(t, storedKCandle.Volume.IsZero())
	})

	t.Run("reports not found when no candle carries that symbol and open time", func(t *testing.T) {
		_, err := kCandleRepository.Update(t.Context(), kCandleAt("BTCUSDT", at(9, 5), "120"))

		assert.ErrorIs(t, err, domains.ErrKCandleNotFound)
	})
}

func TestDelete(t *testing.T) {
	kCandleRepository := persistence.NewKCandleRepository(newTestDatabase(t))
	_, err := kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, err)

	t.Run("removes the named candle", func(t *testing.T) {
		err := kCandleRepository.Delete(t.Context(), "BTCUSDT", at(9, 0))
		assert.NoError(t, err)

		_, err = kCandleRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
		assert.ErrorIs(t, err, domains.ErrKCandleNotFound)
	})

	t.Run("reports not found when no candle carries that symbol and open time", func(t *testing.T) {
		err := kCandleRepository.Delete(t.Context(), "BTCUSDT", at(9, 5))

		assert.ErrorIs(t, err, domains.ErrKCandleNotFound)
	})
}

func TestEveryOperationReportsAnUnusableStore(t *testing.T) {
	database := newTestDatabase(t)
	kCandleRepository := persistence.NewKCandleRepository(database)

	sqlDatabase, err := database.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDatabase.Close())

	t.Run("save", func(t *testing.T) {
		_, err := kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "100"))
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrKCandleNotFound)
	})

	t.Run("update", func(t *testing.T) {
		_, err := kCandleRepository.Update(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "100"))
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrKCandleNotFound)
	})

	t.Run("read one", func(t *testing.T) {
		_, err := kCandleRepository.FindOne(t.Context(), "BTCUSDT", at(9, 0))
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrKCandleNotFound)
	})

	t.Run("read a range", func(t *testing.T) {
		storedKCandles, err := kCandleRepository.FindInRange(t.Context(), queryFor(t, "BTCUSDT", at(9, 0), at(9, 10)), 10)
		assert.Error(t, err)
		assert.Nil(t, storedKCandles)
	})

	t.Run("read the latest", func(t *testing.T) {
		storedKCandles, err := kCandleRepository.FindLatest(t.Context(), "BTCUSDT", 5)
		assert.Error(t, err)
		assert.Nil(t, storedKCandles)
	})

	t.Run("delete", func(t *testing.T) {
		err := kCandleRepository.Delete(t.Context(), "BTCUSDT", at(9, 0))
		assert.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrKCandleNotFound)
	})
}

func TestFindDistinctSymbols(t *testing.T) {
	t.Run("returns every symbol that has a stored candle, each once, by name", func(t *testing.T) {
		repository := persistence.NewKCandleRepository(newTestDatabase(t))
		_, saveError := repository.Save(t.Context(), kCandleAt("SOLUSDT", at(9, 0), "100"))
		require.NoError(t, saveError)
		_, saveError = repository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "100"))
		require.NoError(t, saveError)
		_, saveError = repository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 5), "101"))
		require.NoError(t, saveError)
		_, saveError = repository.Save(t.Context(), kCandleAt("ETHUSDT", at(9, 0), "100"))
		require.NoError(t, saveError)

		symbols, findError := repository.FindDistinctSymbols(t.Context())

		assert.NoError(t, findError)
		assert.Equal(t, []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}, symbols)
	})

	t.Run("returns an empty list when nothing is stored", func(t *testing.T) {
		repository := persistence.NewKCandleRepository(newTestDatabase(t))

		symbols, findError := repository.FindDistinctSymbols(t.Context())

		assert.NoError(t, findError)
		assert.Empty(t, symbols)
	})

	t.Run("stops naming a symbol once its candles are gone", func(t *testing.T) {
		repository := persistence.NewKCandleRepository(newTestDatabase(t))
		_, saveError := repository.Save(t.Context(), kCandleAt("BTCUSDT", at(9, 0), "100"))
		require.NoError(t, saveError)
		_, saveError = repository.Save(t.Context(), kCandleAt("ETHUSDT", at(9, 0), "100"))
		require.NoError(t, saveError)
		require.NoError(t, repository.Delete(t.Context(), "ETHUSDT", at(9, 0)))

		symbols, findError := repository.FindDistinctSymbols(t.Context())

		assert.NoError(t, findError)
		assert.Equal(t, []string{"BTCUSDT"}, symbols)
	})
}

func TestFindLatest(t *testing.T) {
	kCandleRepository := persistence.NewKCandleRepository(newTestDatabase(t))
	for _, openTime := range []time.Time{at(9, 5), at(9, 15), at(9, 0), at(9, 10)} {
		_, err := kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", openTime, "100"))
		require.NoError(t, err)
	}
	_, err := kCandleRepository.Save(t.Context(), kCandleAt("ETHUSDT", at(9, 20), "200"))
	require.NoError(t, err)

	testCases := []struct {
		name              string
		symbol            string
		limit             int
		expectedOpenTimes []time.Time
	}{
		{
			name:              "returns the latest candles newest first",
			symbol:            "BTCUSDT",
			limit:             2,
			expectedOpenTimes: []time.Time{at(9, 15), at(9, 10)},
		},
		{
			name:              "returns every candle when the limit exceeds what is stored",
			symbol:            "BTCUSDT",
			limit:             10,
			expectedOpenTimes: []time.Time{at(9, 15), at(9, 10), at(9, 5), at(9, 0)},
		},
		{
			name:              "keeps trading symbols apart",
			symbol:            "ETHUSDT",
			limit:             10,
			expectedOpenTimes: []time.Time{at(9, 20)},
		},
		{
			name:              "returns nothing for a symbol with no candles",
			symbol:            "SOLUSDT",
			limit:             10,
			expectedOpenTimes: []time.Time{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			storedKCandles, err := kCandleRepository.FindLatest(t.Context(), testCase.symbol, testCase.limit)

			assert.NoError(t, err)
			assert.Len(t, storedKCandles, len(testCase.expectedOpenTimes))
			for index, expectedOpenTime := range testCase.expectedOpenTimes {
				assert.Equal(t, expectedOpenTime, storedKCandles[index].OpenTime.UTC())
			}
		})
	}
}

func TestSavingABatchOverwritesWhatIsHeldAndAddsWhatIsNotWithoutDuplicating(t *testing.T) {
	database := newTestDatabase(t)
	kCandleRepository := persistence.NewKCandleRepository(database)
	for _, openTime := range []time.Time{at(8, 50), at(8, 55), at(9, 0)} {
		_, saveError := kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", openTime, "100"))
		require.NoError(t, saveError)
	}

	batch := []time.Time{at(8, 50), at(8, 55), at(9, 0), at(9, 5), at(9, 10)}
	for _, openTime := range batch {
		_, saveError := kCandleRepository.Save(t.Context(), kCandleAt("BTCUSDT", openTime, "120"))
		require.NoError(t, saveError)
	}

	stored, findError := kCandleRepository.FindInRange(t.Context(), queryFor(t, "BTCUSDT", at(8, 50), at(9, 10)), 100)
	require.NoError(t, findError)
	storedOpenTimes := make([]time.Time, 0, len(stored))
	for _, kCandle := range stored {
		storedOpenTimes = append(storedOpenTimes, kCandle.OpenTime.UTC())
		assert.Equal(t, "120", kCandle.Close.String())
	}
	assert.Equal(t, batch, storedOpenTimes)
}

func TestFindDistinctSymbolsStorageFailure(t *testing.T) {
	repository := persistence.NewKCandleRepository(closedDatabase(t))

	_, findError := repository.FindDistinctSymbols(t.Context())

	assert.Error(t, findError)
}
