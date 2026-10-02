package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aSnapshot(symbol string, closePrice string, observedAt time.Time) entities.LiveKCandleSnapshot {
	return entities.LiveKCandleSnapshot{
		Symbol: symbol, OpenTime: time.Date(2026, 10, 3, 1, 59, 0, 0, time.UTC),
		Open: decimal.RequireFromString("1000"), High: decimal.RequireFromString("1010"),
		Low: decimal.RequireFromString("990"), Close: decimal.RequireFromString(closePrice),
		Volume: decimal.RequireFromString("12"), ObservedAt: observedAt,
	}
}

func TestLiveKCandleSnapshotRepositoryKeepsTheLatestPerSymbol(t *testing.T) {
	repository := persistence.NewLiveKCandleSnapshotRepository(newTestDatabase(t))
	seenAt := time.Date(2026, 10, 3, 2, 0, 0, 0, time.UTC)
	require.NoError(t, repository.Save(t.Context(), aSnapshot("2330", "1005", seenAt)))
	require.NoError(t, repository.Save(t.Context(), aSnapshot("2330", "1007", seenAt.Add(5*time.Second))))
	require.NoError(t, repository.Save(t.Context(), aSnapshot("2454", "800", seenAt)))

	snapshots, findError := repository.FindBySymbols(t.Context(), []string{"2330"})
	require.NoError(t, findError)

	require.Len(t, snapshots, 1)
	assert.Equal(t, "1007", snapshots[0].Close.String())
	assert.True(t, seenAt.Add(5*time.Second).Equal(snapshots[0].ObservedAt))
}

func TestLiveKCandleSnapshotRepositoryFindsNothingForNoSymbols(t *testing.T) {
	repository := persistence.NewLiveKCandleSnapshotRepository(newTestDatabase(t))

	snapshots, findError := repository.FindBySymbols(t.Context(), nil)

	require.NoError(t, findError)
	assert.Empty(t, snapshots)
}

func TestLiveKCandleSnapshotRepositorySaysSoWhenStorageIsGone(t *testing.T) {
	repository := persistence.NewLiveKCandleSnapshotRepository(closedDatabase(t))

	assert.Error(t, repository.Save(t.Context(), aSnapshot("2330", "1005", time.Now())))
	_, findError := repository.FindBySymbols(t.Context(), []string{"2330"})
	assert.Error(t, findError)
}
