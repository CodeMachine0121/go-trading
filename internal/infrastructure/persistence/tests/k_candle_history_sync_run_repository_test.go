package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startedRun is a sync just accepted: recorded before a single candle is fetched,
// which is what makes the work findable while it is still going.
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
	// Progress is written over and over as the run walks the stretch. A second row per
	// update would turn one run into thousands of them.
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
	// Zero is a real answer for every count here, and a write that treats it as
	// "nothing to say" would leave the previous figure standing.
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

func TestLookingUpAHistorySyncNobodyStartedIsNotAnError(t *testing.T) {
	repository := persistence.NewKCandleHistorySyncRunRepository(newTestDatabase(t))

	_, exists, findError := repository.FindOne(t.Context(), 4242)

	require.NoError(t, findError)
	assert.False(t, exists)
}

func TestFailingTheRunningHistorySyncsLeavesTheFinishedOnesAlone(t *testing.T) {
	// Every run still recorded as fetching is stale the moment the process restarts:
	// nothing is fetching for it. The ones that already ended are history and must
	// not be rewritten.
	repository := persistence.NewKCandleHistorySyncRunRepository(newTestDatabase(t))
	interrupted, firstError := repository.Save(t.Context(), startedRun("BTCUSDT", 30))
	require.NoError(t, firstError)

	finishedAt := time.Date(2026, 9, 7, 2, 0, 0, 0, time.UTC)
	succeeded := startedRun("ETHUSDT", 30)
	succeeded.Status = string(vo.KCandleHistorySyncSucceeded)
	succeeded.FinishedAt = &finishedAt
	succeeded, secondError := repository.Save(t.Context(), succeeded)
	require.NoError(t, secondError)

	sweptCount, sweepError := repository.FailAllRunning(t.Context(), "interrupted by restart")

	require.NoError(t, sweepError)
	assert.Equal(t, 1, sweptCount)

	foundInterrupted, _, _ := repository.FindOne(t.Context(), interrupted.ID)
	assert.Equal(t, string(vo.KCandleHistorySyncFailed), foundInterrupted.Status)
	assert.Equal(t, "interrupted by restart", foundInterrupted.FailureReason)

	foundSucceeded, _, _ := repository.FindOne(t.Context(), succeeded.ID)
	assert.Equal(t, string(vo.KCandleHistorySyncSucceeded), foundSucceeded.Status)
}

func TestAHistorySyncRepositorySaysSoWhenStorageIsGone(t *testing.T) {
	repository := persistence.NewKCandleHistorySyncRunRepository(closedDatabase(t))

	_, saveError := repository.Save(t.Context(), startedRun("BTCUSDT", 30))
	require.Error(t, saveError)

	_, _, findError := repository.FindOne(t.Context(), 1)
	require.Error(t, findError)

	_, sweepError := repository.FailAllRunning(t.Context(), "interrupted by restart")
	require.Error(t, sweepError)
}
