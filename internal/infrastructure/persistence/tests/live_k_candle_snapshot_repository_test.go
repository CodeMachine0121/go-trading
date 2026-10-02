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
	return aSnapshotOf(symbol, time.Date(2026, 10, 3, 1, 59, 0, 0, time.UTC), closePrice, observedAt)
}

func aSnapshotOf(symbol string, openTime time.Time, closePrice string, observedAt time.Time) entities.LiveKCandleSnapshot {
	return entities.LiveKCandleSnapshot{
		Symbol: symbol, OpenTime: openTime,
		Open: decimal.RequireFromString("1000"), High: decimal.RequireFromString("1010"),
		Low: decimal.RequireFromString("990"), Close: decimal.RequireFromString(closePrice),
		Volume: decimal.RequireFromString("12"), ObservedAt: observedAt,
	}
}

func TestLiveKCandleSnapshotRepositoryKeepsTheLatestSightingOfEachMinute(t *testing.T) {
	repository := persistence.NewLiveKCandleSnapshotRepository(newTestDatabase(t))
	seenAt := time.Date(2026, 10, 3, 2, 0, 0, 0, time.UTC)
	closingMinute := time.Date(2026, 10, 3, 1, 59, 0, 0, time.UTC)
	nextMinute := time.Date(2026, 10, 3, 2, 0, 0, 0, time.UTC)
	require.NoError(t, repository.Save(t.Context(), aSnapshotOf("2330", closingMinute, "1005", seenAt)))
	require.NoError(t, repository.Save(t.Context(), aSnapshotOf("2330", closingMinute, "1007", seenAt.Add(time.Second))))
	require.NoError(t, repository.Save(t.Context(), aSnapshotOf("2330", nextMinute, "1008", seenAt.Add(2*time.Second))))
	require.NoError(t, repository.Save(t.Context(), aSnapshotOf("2454", closingMinute, "800", seenAt)))

	snapshots, findError := repository.FindObservedAfter(t.Context(), []string{"2330"}, seenAt)
	require.NoError(t, findError)

	// The closed minute is not overwritten by the next one, and comes first.
	require.Len(t, snapshots, 2)
	assert.Equal(t, "1007", snapshots[0].Close.String())
	assert.Equal(t, "1008", snapshots[1].Close.String())
}

func TestLiveKCandleSnapshotRepositoryFindsOnlyWhatWasSeenAfterACursor(t *testing.T) {
	repository := persistence.NewLiveKCandleSnapshotRepository(newTestDatabase(t))
	seenAt := time.Date(2026, 10, 3, 2, 0, 0, 0, time.UTC)
	require.NoError(t, repository.Save(t.Context(), aSnapshot("2330", "1005", seenAt)))

	atTheCursor, findError := repository.FindObservedAfter(t.Context(), []string{"2330"}, seenAt)
	require.NoError(t, findError)
	nothingAsked, emptyError := repository.FindObservedAfter(t.Context(), nil, seenAt.Add(-time.Hour))
	require.NoError(t, emptyError)

	assert.Empty(t, atTheCursor)
	assert.Empty(t, nothingAsked)
}

func TestLiveKCandleSnapshotRepositoryDropsOnlyOldSightings(t *testing.T) {
	repository := persistence.NewLiveKCandleSnapshotRepository(newTestDatabase(t))
	seenAt := time.Date(2026, 10, 3, 2, 0, 0, 0, time.UTC)
	require.NoError(t, repository.Save(t.Context(), aSnapshotOf("2330", seenAt.Add(-time.Hour), "990", seenAt.Add(-time.Hour))))
	require.NoError(t, repository.Save(t.Context(), aSnapshot("2330", "1005", seenAt)))

	require.NoError(t, repository.DeleteObservedBefore(t.Context(), seenAt.Add(-10*time.Minute)))

	left, findError := repository.FindObservedAfter(t.Context(), []string{"2330"}, seenAt.Add(-2*time.Hour))
	require.NoError(t, findError)
	require.Len(t, left, 1)
	assert.Equal(t, "1005", left[0].Close.String())
}

func TestLiveKCandleSnapshotRepositorySaysSoWhenStorageIsGone(t *testing.T) {
	repository := persistence.NewLiveKCandleSnapshotRepository(closedDatabase(t))

	assert.Error(t, repository.Save(t.Context(), aSnapshot("2330", "1005", time.Now())))
	_, findError := repository.FindObservedAfter(t.Context(), []string{"2330"}, time.Now())
	assert.Error(t, findError)
	assert.Error(t, repository.DeleteObservedBefore(t.Context(), time.Now()))
}
