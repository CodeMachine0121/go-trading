package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// clearAppliedDataRetirements puts the ledger back to how a database that has never
// run a retirement looks, which is the state every case here starts from.
func clearAppliedDataRetirements(t *testing.T, database *gorm.DB) {
	t.Helper()

	require.NoError(t, database.
		Session(&gorm.Session{AllowGlobalUpdate: true}).
		WithContext(t.Context()).
		Delete(&entities.AppliedDataRetirement{}).Error)
}

func newRetirementUnderTest(t *testing.T) (*persistence.DataRetirementMigrator, *gorm.DB) {
	t.Helper()

	database := newTestDatabase(t)
	clearAppliedDataRetirements(t, database)

	return persistence.NewDataRetirementMigrator(
		database, persistence.NewKCandleRepository(database)), database
}

func TestRetiringEmptiesTheKCandlesStoredAtTheOldLength(t *testing.T) {
	migrator, database := newRetirementUnderTest(t)
	require.NoError(t, database.WithContext(t.Context()).
		Create(&[]entities.KCandle{
			kCandleRow("BTCUSDT", "2026-08-29T09:00:00Z"),
			kCandleRow("BTCUSDT", "2026-08-29T09:05:00Z"),
		}).Error)

	appliedNames, retireError := migrator.Retire(t.Context())

	require.NoError(t, retireError)
	assert.Equal(t,
		[]string{persistence.KCandlesBeforeOneMinuteGranularityRetirement}, appliedNames)
	assert.Equal(t, int64(0), countOf(t, database, &entities.KCandle{}))
}

func TestRetiringWithNothingStoredIsNotAFailure(t *testing.T) {
	migrator, database := newRetirementUnderTest(t)

	appliedNames, retireError := migrator.Retire(t.Context())

	require.NoError(t, retireError)
	assert.Equal(t,
		[]string{persistence.KCandlesBeforeOneMinuteGranularityRetirement}, appliedNames)
	assert.Equal(t, int64(0), countOf(t, database, &entities.KCandle{}))
}

func TestRetiringASecondTimeLeavesTheCandlesFetchedSinceAlone(t *testing.T) {
	// This is the whole reason the ledger exists. A second run over a store the
	// backfill has already refilled at the new length must find nothing to do —
	// emptying it again would cost every candle fetched since.
	migrator, database := newRetirementUnderTest(t)
	_, firstError := migrator.Retire(t.Context())
	require.NoError(t, firstError)

	require.NoError(t, database.WithContext(t.Context()).
		Create(&[]entities.KCandle{
			kCandleRow("BTCUSDT", "2026-08-29T09:00:00Z"),
			kCandleRow("BTCUSDT", "2026-08-29T09:01:00Z"),
		}).Error)

	appliedNames, secondError := migrator.Retire(t.Context())

	require.NoError(t, secondError)
	assert.Empty(t, appliedNames)
	assert.Equal(t, int64(2), countOf(t, database, &entities.KCandle{}))
}

func TestRetiringLeavesEverythingThatIsNotAKCandleAlone(t *testing.T) {
	migrator, database := newRetirementUnderTest(t)
	require.NoError(t, database.WithContext(t.Context()).
		Create(&entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: "crypto", IsWatched: true,
		}).Error)
	require.NoError(t, database.WithContext(t.Context()).
		Create(&entities.Strategy{
			Name: "a strategy", Script: "the script", ResultType: "float",
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}).Error)

	_, retireError := migrator.Retire(t.Context())

	require.NoError(t, retireError)
	assert.Equal(t, int64(1), countOf(t, database, &entities.TradingSymbol{}))
	assert.Equal(t, int64(1), countOf(t, database, &entities.Strategy{}))
}

func TestDeletingEveryKCandleReportsHowManyItRemoved(t *testing.T) {
	database := newTestDatabase(t)
	require.NoError(t, database.WithContext(t.Context()).
		Create(&[]entities.KCandle{
			kCandleRow("BTCUSDT", "2026-08-29T09:00:00Z"),
			kCandleRow("ETHUSDT", "2026-08-29T09:01:00Z"),
			kCandleRow("ETHUSDT", "2026-08-29T09:02:00Z"),
		}).Error)

	removedCount, deleteError := persistence.NewKCandleRepository(database).DeleteAll(t.Context())

	require.NoError(t, deleteError)
	assert.Equal(t, int64(3), removedCount)
	assert.Equal(t, int64(0), countOf(t, database, &entities.KCandle{}))
}

func TestDeletingEveryKCandleFromAnEmptyStoreRemovesNone(t *testing.T) {
	database := newTestDatabase(t)

	removedCount, deleteError := persistence.NewKCandleRepository(database).DeleteAll(t.Context())

	require.NoError(t, deleteError)
	assert.Equal(t, int64(0), removedCount)
}

func countOf(t *testing.T, database *gorm.DB, model any) int64 {
	t.Helper()

	storedCount := int64(0)
	require.NoError(t, database.WithContext(t.Context()).Model(model).Count(&storedCount).Error)

	return storedCount
}

func kCandleRow(symbol string, openTime string) entities.KCandle {
	parsedOpenTime, parseError := time.Parse(time.RFC3339, openTime)
	if parseError != nil {
		panic(parseError)
	}

	return entities.KCandle{
		Symbol:   symbol,
		OpenTime: parsedOpenTime,
		Open:     decimal.RequireFromString("100"),
		High:     decimal.RequireFromString("120"),
		Low:      decimal.RequireFromString("90"),
		Close:    decimal.RequireFromString("110"),
		Volume:   decimal.RequireFromString("11"),
	}
}
