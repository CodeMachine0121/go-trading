package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startedRun is a sync recorded before any candle is fetched.
func startedRun(symbol string, totalChunks int) entities.KCandleHistorySyncRun {
	return entities.KCandleHistorySyncRun{
		Symbol:       symbol,
		LookbackDays: 30,
		Status:       string(vo.KCandleHistorySyncRunning),
		TotalChunks:  totalChunks,
		StartedAt:    time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC),
	}
}

func TestSavingAHistorySyncGivesItSomewhereToBeLookedUp(t *testing.T) {
	repository := persistence.NewKCandleHistorySyncRunRepository(newTestDatabase(t))

	saved, saveError := repository.Save(t.Context(), startedRun("BTCUSDT", 30))

	require.NoError(t, saveError)
	require.NotZero(t, saved.ID)

	found, exists, findError := repository.FindOne(t.Context(), saved.ID)

	require.NoError(t, findError)
	require.True(t, exists)
	assert.Equal(t, "BTCUSDT", found.Symbol)
	assert.Equal(t, 30, found.TotalChunks)
	assert.Equal(t, string(vo.KCandleHistorySyncRunning), found.Status)
	assert.Nil(t, found.FinishedAt)
}

func TestSavingAHistorySyncAgainMovesItAlongInsteadOfMakingASecondOne(t *testing.T) {
	// Progress updates must rewrite the same row, not add rows.
	repository := persistence.NewKCandleHistorySyncRunRepository(newTestDatabase(t))
	saved, saveError := repository.Save(t.Context(), startedRun("BTCUSDT", 30))
	require.NoError(t, saveError)

	saved.CompletedChunks = 11
	saved.StoredCount = 15840
	_, progressError := repository.Save(t.Context(), saved)
	require.NoError(t, progressError)

	found, _, findError := repository.FindOne(t.Context(), saved.ID)

	require.NoError(t, findError)
	assert.Equal(t, 11, found.CompletedChunks)
	assert.Equal(t, 15840, found.StoredCount)
}

func TestAHistorySyncGoingBackToNothingStoredIsWrittenDownAsSuch(t *testing.T) {
	// Zero counts must overwrite previous values rather than be skipped as empty.
	repository := persistence.NewKCandleHistorySyncRunRepository(newTestDatabase(t))
	saved, saveError := repository.Save(t.Context(), startedRun("BTCUSDT", 30))
	require.NoError(t, saveError)

	saved.StoredCount = 42
	_, firstError := repository.Save(t.Context(), saved)
	require.NoError(t, firstError)

	saved.StoredCount = 0
	_, secondError := repository.Save(t.Context(), saved)
	require.NoError(t, secondError)

	found, _, findError := repository.FindOne(t.Context(), saved.ID)

	require.NoError(t, findError)
	assert.Equal(t, 0, found.StoredCount)
}

func TestOneSymbolCannotBeFetchedByTwoHistorySyncsAtOnce(t *testing.T) {
	// A duplicate running sync per symbol wastes hours of quota, and the database index rather than a read must prevent it.
	repository := persistence.NewKCandleHistorySyncRunRepository(newTestDatabase(t))
	_, firstError := repository.Save(t.Context(), startedRun("BTCUSDT", 30))
	require.NoError(t, firstError)

	_, secondError := repository.Save(t.Context(), startedRun("BTCUSDT", 30))

	require.ErrorIs(t, secondError, domains.ErrKCandleHistorySyncInProgress)
	assert.Contains(t, secondError.Error(), "BTCUSDT")
}

func TestAnotherSymbolMayBeFetchedWhileOneIsRunning(t *testing.T) {
	repository := persistence.NewKCandleHistorySyncRunRepository(newTestDatabase(t))
	_, firstError := repository.Save(t.Context(), startedRun("BTCUSDT", 30))
	require.NoError(t, firstError)

	_, secondError := repository.Save(t.Context(), startedRun("ETHUSDT", 30))

	require.NoError(t, secondError)
}

func TestASymbolMayBeFetchedAgainOnceTheRunBeforeItEnded(t *testing.T) {
	// Only a running sync blocks the symbol; finished ones do not.
	repository := persistence.NewKCandleHistorySyncRunRepository(newTestDatabase(t))
	first, firstError := repository.Save(t.Context(), startedRun("BTCUSDT", 30))
	require.NoError(t, firstError)

	finishedAt := time.Date(2026, 9, 7, 2, 0, 0, 0, time.UTC)
	first.Status = string(vo.KCandleHistorySyncSucceeded)
	first.FinishedAt = &finishedAt
	_, endError := repository.Save(t.Context(), first)
	require.NoError(t, endError)

	_, secondError := repository.Save(t.Context(), startedRun("BTCUSDT", 30))

	require.NoError(t, secondError)
}

func TestLookingUpAHistorySyncNobodyStartedIsNotAnError(t *testing.T) {
	repository := persistence.NewKCandleHistorySyncRunRepository(newTestDatabase(t))

	_, exists, findError := repository.FindOne(t.Context(), 4242)

	require.NoError(t, findError)
	assert.False(t, exists)
}

func TestFailingTheRunningHistorySyncsLeavesTheFinishedOnesAlone(t *testing.T) {
	// A restart marks still-fetching runs interrupted and leaves finished ones untouched.
	repository := persistence.NewKCandleHistorySyncRunRepository(newTestDatabase(t))
	interrupted, firstError := repository.Save(t.Context(), startedRun("BTCUSDT", 30))
	require.NoError(t, firstError)

	finishedAt := time.Date(2026, 9, 7, 2, 0, 0, 0, time.UTC)
	succeeded := startedRun("ETHUSDT", 30)
	succeeded.Status = string(vo.KCandleHistorySyncSucceeded)
	succeeded.FinishedAt = &finishedAt
	succeeded, secondError := repository.Save(t.Context(), succeeded)
	require.NoError(t, secondError)

	sweptAt := time.Date(2026, 9, 7, 3, 0, 0, 0, time.UTC)
	sweptCount, sweepError := repository.FailAllRunning(
		t.Context(), "interrupted by restart", sweptAt)

	require.NoError(t, sweepError)
	assert.Equal(t, 1, sweptCount)

	foundInterrupted, _, _ := repository.FindOne(t.Context(), interrupted.ID)
	assert.Equal(t, string(vo.KCandleHistorySyncFailed), foundInterrupted.Status)
	assert.Equal(t, "interrupted by restart", foundInterrupted.FailureReason)
	// A swept run must get a finish time, or it reads as still in flight.
	require.NotNil(t, foundInterrupted.FinishedAt)
	assert.Equal(t, sweptAt, foundInterrupted.FinishedAt.UTC())

	foundSucceeded, _, _ := repository.FindOne(t.Context(), succeeded.ID)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), foundSucceeded.Status)
}

func TestAHistorySyncRepositorySaysSoWhenStorageIsGone(t *testing.T) {
	repository := persistence.NewKCandleHistorySyncRunRepository(closedDatabase(t))

	_, saveError := repository.Save(t.Context(), startedRun("BTCUSDT", 30))
	require.Error(t, saveError)

	_, _, findError := repository.FindOne(t.Context(), 1)
	require.Error(t, findError)

	_, sweepError := repository.FailAllRunning(
		t.Context(), "interrupted by restart", time.Date(2026, 9, 7, 3, 0, 0, 0, time.UTC))
	require.Error(t, sweepError)
}
