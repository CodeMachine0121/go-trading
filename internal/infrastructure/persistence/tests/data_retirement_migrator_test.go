package persistence_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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

func TestRetirementWorkThatFailsIsReportedAndNotRememberedAsDone(t *testing.T) {
	// Remembering a retirement that did not happen is the one outcome the ledger must
	// never produce: nothing would ever remove that data again.
	database := newTestDatabase(t)
	clearAppliedDataRetirements(t, database)
	kCandleRepository := mocks.NewMockIKCandleRepository(gomock.NewController(t))
	storageFailure := errors.New("storage refused the delete")
	kCandleRepository.EXPECT().DeleteAll(gomock.Any()).Return(int64(0), storageFailure)

	appliedNames, retireError := persistence.
		NewDataRetirementMigrator(database, kCandleRepository).
		Retire(t.Context())

	require.ErrorIs(t, retireError, storageFailure)
	assert.Nil(t, appliedNames)
	assert.Equal(t, int64(0), countOf(t, database, &entities.AppliedDataRetirement{}))
}

func TestALedgerThatCannotBeReadStopsTheRetirement(t *testing.T) {
	// Not knowing whether a retirement has run is not the same as knowing it has not.
	// Guessing "not yet" would empty a store that was already refilled.
	database := newTestDatabase(t)
	clearAppliedDataRetirements(t, database)
	kCandleRepository := mocks.NewMockIKCandleRepository(gomock.NewController(t))
	connection, connectionError := database.DB()
	require.NoError(t, connectionError)
	require.NoError(t, connection.Close())

	appliedNames, retireError := persistence.
		NewDataRetirementMigrator(database, kCandleRepository).
		Retire(t.Context())

	require.Error(t, retireError)
	assert.Contains(t, retireError.Error(), "read applied data retirements")
	assert.Nil(t, appliedNames)
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

func TestDeletingEveryKCandleReportsAStorageFailureRatherThanACount(t *testing.T) {
	// A delete that never reached the database has removed nothing, and answering
	// zero would be indistinguishable from an empty store — which the caller reads
	// as work already done.
	database := newTestDatabase(t)
	connection, connectionError := database.DB()
	require.NoError(t, connectionError)
	require.NoError(t, connection.Close())

	removedCount, deleteError := persistence.NewKCandleRepository(database).DeleteAll(t.Context())

	require.Error(t, deleteError)
	assert.Contains(t, deleteError.Error(), "delete every k candle")
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
