package persistence_test

import (
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func storedTier(symbol string, tier int, floor string, cap string, confirmedAt time.Time) entities.ContractMaintenanceMarginTier {
	return entities.ContractMaintenanceMarginTier{
		Symbol:                symbol,
		Tier:                  tier,
		NotionalFloor:         decimal.RequireFromString(floor),
		NotionalCap:           decimal.RequireFromString(cap),
		MaintenanceMarginRate: decimal.RequireFromString("0.004"),
		MaintenanceAmount:     decimal.Zero,
		MaximumLeverage:       125,
		ConfirmedAt:           confirmedAt,
	}
}

func TestContractMaintenanceMarginTierRepositoryReplacesALadderWhole(t *testing.T) {
	database := newTestDatabase(t)
	tierRepository := persistence.NewContractMaintenanceMarginTierRepository(database)
	require.NoError(t, tierRepository.ReplaceLadders(t.Context(), map[string][]entities.ContractMaintenanceMarginTier{
		"BTCUSDT": {storedTier("BTCUSDT", 3, "250000", "1000000", at(8, 0)),
			storedTier("BTCUSDT", 1, "0", "50000", at(8, 0)), storedTier("BTCUSDT", 2, "50000", "250000", at(8, 0))},
		"ETHUSDT": {storedTier("ETHUSDT", 1, "0", "10000", at(8, 0))},
	}))

	replaceError := tierRepository.ReplaceLadders(t.Context(), map[string][]entities.ContractMaintenanceMarginTier{
		"BTCUSDT": {storedTier("BTCUSDT", 1, "0", "60000", at(9, 0)), storedTier("BTCUSDT", 2, "60000", "300000", at(9, 0))},
	})

	require.NoError(t, replaceError)
	bitcoin, findError := tierRepository.FindBySymbol(t.Context(), "BTCUSDT")
	require.NoError(t, findError)
	require.Len(t, bitcoin, 2, "新的一組少一級，舊的第三級也要跟著不見")
	assert.Equal(t, 1, bitcoin[0].Tier)
	assert.True(t, decimal.RequireFromString("60000").Equal(bitcoin[0].NotionalCap))
	assert.Equal(t, at(9, 0), bitcoin[1].ConfirmedAt.UTC())
	ether, _ := tierRepository.FindBySymbol(t.Context(), "ETHUSDT")
	require.Len(t, ether, 1, "沒被點名的標的保留原本的分級")
	assert.Equal(t, at(8, 0), ether[0].ConfirmedAt.UTC())
}

func TestContractMaintenanceMarginTierRepositoryReadsTiersFirstFirst(t *testing.T) {
	database := newTestDatabase(t)
	tierRepository := persistence.NewContractMaintenanceMarginTierRepository(database)
	require.NoError(t, tierRepository.ReplaceLadders(t.Context(), map[string][]entities.ContractMaintenanceMarginTier{
		"BTCUSDT": {storedTier("BTCUSDT", 2, "50000", "250000", at(8, 0)), storedTier("BTCUSDT", 1, "0", "50000", at(8, 0))},
	}))

	bitcoin, findError := tierRepository.FindBySymbol(t.Context(), "BTCUSDT")
	nothing, nothingError := tierRepository.FindBySymbol(t.Context(), "ETHUSDT")

	require.NoError(t, findError)
	require.Len(t, bitcoin, 2)
	assert.Equal(t, 1, bitcoin[0].Tier)
	assert.Equal(t, 2, bitcoin[1].Tier)
	require.NoError(t, nothingError)
	assert.Empty(t, nothing)
}

func TestContractMaintenanceMarginTierRepositoryReplacesNothingWhenOneLadderCannotBeWritten(t *testing.T) {
	database := newTestDatabase(t)
	tierRepository := persistence.NewContractMaintenanceMarginTierRepository(database)
	require.NoError(t, tierRepository.ReplaceLadders(t.Context(), map[string][]entities.ContractMaintenanceMarginTier{
		"BTCUSDT": {storedTier("BTCUSDT", 1, "0", "50000", at(8, 0))},
	}))

	// Two tiers with the same number break the unique key partway through.
	replaceError := tierRepository.ReplaceLadders(t.Context(), map[string][]entities.ContractMaintenanceMarginTier{
		"BTCUSDT": {storedTier("BTCUSDT", 1, "0", "60000", at(9, 0)), storedTier("BTCUSDT", 1, "60000", "90000", at(9, 0))},
	})

	assert.ErrorContains(t, replaceError, "replace contract maintenance margin ladders")
	bitcoin, _ := tierRepository.FindBySymbol(t.Context(), "BTCUSDT")
	require.Len(t, bitcoin, 1)
	assert.Equal(t, at(8, 0), bitcoin[0].ConfirmedAt.UTC())
}

func TestContractMaintenanceMarginTierRepositoryClearsALadderGivenNoTiers(t *testing.T) {
	database := newTestDatabase(t)
	tierRepository := persistence.NewContractMaintenanceMarginTierRepository(database)
	require.NoError(t, tierRepository.ReplaceLadders(t.Context(), map[string][]entities.ContractMaintenanceMarginTier{
		"BTCUSDT": {storedTier("BTCUSDT", 1, "0", "50000", at(8, 0))},
	}))

	require.NoError(t, tierRepository.ReplaceLadders(t.Context(),
		map[string][]entities.ContractMaintenanceMarginTier{"BTCUSDT": {}}))

	bitcoin, _ := tierRepository.FindBySymbol(t.Context(), "BTCUSDT")
	assert.Empty(t, bitcoin)
}

func TestContractMaintenanceMarginTierRepositorySaysSoWhenStorageIsUnreachable(t *testing.T) {
	database := newTestDatabase(t)
	tierRepository := persistence.NewContractMaintenanceMarginTierRepository(database)
	connection, connectionError := database.DB()
	require.NoError(t, connectionError)
	require.NoError(t, connection.Close())

	replaceError := tierRepository.ReplaceLadders(t.Context(), map[string][]entities.ContractMaintenanceMarginTier{
		"BTCUSDT": {storedTier("BTCUSDT", 1, "0", "50000", at(8, 0))},
	})
	_, findError := tierRepository.FindBySymbol(t.Context(), "BTCUSDT")

	assert.Error(t, replaceError)
	assert.Error(t, findError)
}

func TestContractMaintenanceMarginTierRepositoryReplacesNothingWhenAnOldLadderCannotBeRemoved(t *testing.T) {
	database := newTestDatabase(t)
	tierRepository := persistence.NewContractMaintenanceMarginTierRepository(database)

	// PostgreSQL refuses a NUL inside text, so removing that contract's old ladder fails.
	replaceError := tierRepository.ReplaceLadders(t.Context(), map[string][]entities.ContractMaintenanceMarginTier{
		"BAD\x00USDT": {storedTier("BAD\x00USDT", 1, "0", "50000", at(8, 0))},
	})

	assert.ErrorContains(t, replaceError, "replace contract maintenance margin ladders")
}

func TestContractMaintenanceMarginTierRepositoryLetsTwoRefreshesMeet(t *testing.T) {
	// A watchlist addition racing the daily refresh: both replace the same ladder concurrently and neither may fail.
	database := newTestDatabase(t)
	tierRepository := persistence.NewContractMaintenanceMarginTierRepository(database)
	ladder := func(confirmedAt time.Time) map[string][]entities.ContractMaintenanceMarginTier {
		return map[string][]entities.ContractMaintenanceMarginTier{"BTCUSDT": {
			storedTier("BTCUSDT", 1, "0", "50000", confirmedAt),
			storedTier("BTCUSDT", 2, "50000", "250000", confirmedAt),
		}}
	}
	require.NoError(t, tierRepository.ReplaceLadders(t.Context(), ladder(at(7, 0))))

	replaceErrors := make(chan error, 16)
	var waitGroup sync.WaitGroup
	for attempt := range 16 {
		waitGroup.Go(func() {
			replaceErrors <- tierRepository.ReplaceLadders(t.Context(), ladder(at(8, attempt)))
		})
	}
	waitGroup.Wait()
	close(replaceErrors)

	for replaceError := range replaceErrors {
		assert.NoError(t, replaceError)
	}
	bitcoin, findError := tierRepository.FindBySymbol(t.Context(), "BTCUSDT")
	require.NoError(t, findError)
	assert.Len(t, bitcoin, 2)
}
