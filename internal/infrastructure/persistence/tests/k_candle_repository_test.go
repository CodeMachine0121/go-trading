package persistence_test

import (
	"os"
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

// newTestDatabase connects to the PostgreSQL instance named by TEST_POSTGRES_DSN and
// starts each test from an empty table. Without that variable the storage behaviors
// cannot be exercised, so the tests skip rather than pretend to pass.
func newTestDatabase(t *testing.T) *gorm.DB {
	dataSourceName := os.Getenv("TEST_POSTGRES_DSN")
	if dataSourceName == "" {
		t.Skip("TEST_POSTGRES_DSN is not set; skipping storage tests")
	}

	database, err := persistence.NewDatabase(dataSourceName)
	require.NoError(t, err)
	_, err = persistence.NewSchemaMigrator(database).Migrate()
	require.NoError(t, err)
	clearedDatabase := database.Session(&gorm.Session{AllowGlobalUpdate: true})
	require.NoError(t, clearedDatabase.Delete(&entities.KCandle{}).Error)

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
	_, err := kCandleRepository.Save(kCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, err)

	_, err = kCandleRepository.Save(kCandleAt("BTCUSDT", at(9, 0), "120"))
	require.NoError(t, err)

	storedKCandles, err := kCandleRepository.FindInRange(queryFor(t, "BTCUSDT", at(9, 0), at(9, 0)), 10)
	assert.NoError(t, err)
	assert.Len(t, storedKCandles, 1)
	assert.True(t, decimal.RequireFromString("120").Equal(storedKCandles[0].Close))
}

func TestSaveKeepsCandlesOfDifferentSymbolsApart(t *testing.T) {
	kCandleRepository := persistence.NewKCandleRepository(newTestDatabase(t))
	_, err := kCandleRepository.Save(kCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, err)
	_, err = kCandleRepository.Save(kCandleAt("ETHUSDT", at(9, 0), "200"))
	require.NoError(t, err)

	storedKCandles, err := kCandleRepository.FindInRange(queryFor(t, "ETHUSDT", at(9, 0), at(9, 0)), 10)
	assert.NoError(t, err)
	assert.Len(t, storedKCandles, 1)
	assert.True(t, decimal.RequireFromString("200").Equal(storedKCandles[0].Close))
}

func TestFindInRange(t *testing.T) {
	kCandleRepository := persistence.NewKCandleRepository(newTestDatabase(t))
	for _, openTime := range []time.Time{at(9, 10), at(9, 0), at(9, 5)} {
		_, err := kCandleRepository.Save(kCandleAt("BTCUSDT", openTime, "100"))
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
			storedKCandles, err := kCandleRepository.FindInRange(
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
	_, err := kCandleRepository.Save(kCandleAt("BTCUSDT", at(9, 0), "110"))
	require.NoError(t, err)

	t.Run("returns the named candle", func(t *testing.T) {
		storedKCandle, err := kCandleRepository.FindOne("BTCUSDT", at(9, 0))

		assert.NoError(t, err)
		assert.True(t, decimal.RequireFromString("110").Equal(storedKCandle.Close))
	})

	t.Run("reports not found when no candle carries that symbol and open time", func(t *testing.T) {
		_, err := kCandleRepository.FindOne("BTCUSDT", at(9, 5))

		assert.ErrorIs(t, err, domains.ErrKCandleNotFound)
	})
}

func TestUpdate(t *testing.T) {
	kCandleRepository := persistence.NewKCandleRepository(newTestDatabase(t))
	_, err := kCandleRepository.Save(kCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, err)

	t.Run("replaces the figures of an existing candle", func(t *testing.T) {
		_, err := kCandleRepository.Update(kCandleAt("BTCUSDT", at(9, 0), "120"))
		assert.NoError(t, err)

		storedKCandle, err := kCandleRepository.FindOne("BTCUSDT", at(9, 0))
		assert.NoError(t, err)
		assert.True(t, decimal.RequireFromString("120").Equal(storedKCandle.Close))
	})

	t.Run("stores a figure of zero rather than skipping it", func(t *testing.T) {
		zeroVolumeKCandle := kCandleAt("BTCUSDT", at(9, 0), "120")
		zeroVolumeKCandle.Volume = decimal.Zero

		_, err := kCandleRepository.Update(zeroVolumeKCandle)
		assert.NoError(t, err)

		storedKCandle, err := kCandleRepository.FindOne("BTCUSDT", at(9, 0))
		assert.NoError(t, err)
		assert.True(t, storedKCandle.Volume.IsZero())
	})

	t.Run("reports not found when no candle carries that symbol and open time", func(t *testing.T) {
		_, err := kCandleRepository.Update(kCandleAt("BTCUSDT", at(9, 5), "120"))

		assert.ErrorIs(t, err, domains.ErrKCandleNotFound)
	})
}

func TestDelete(t *testing.T) {
	kCandleRepository := persistence.NewKCandleRepository(newTestDatabase(t))
	_, err := kCandleRepository.Save(kCandleAt("BTCUSDT", at(9, 0), "100"))
	require.NoError(t, err)

	t.Run("removes the named candle", func(t *testing.T) {
		err := kCandleRepository.Delete("BTCUSDT", at(9, 0))
		assert.NoError(t, err)

		_, err = kCandleRepository.FindOne("BTCUSDT", at(9, 0))
		assert.ErrorIs(t, err, domains.ErrKCandleNotFound)
	})

	t.Run("reports not found when no candle carries that symbol and open time", func(t *testing.T) {
		err := kCandleRepository.Delete("BTCUSDT", at(9, 5))

		assert.ErrorIs(t, err, domains.ErrKCandleNotFound)
	})
}
