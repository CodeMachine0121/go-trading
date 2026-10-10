package persistence_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	backtestRunOwnerID      = uint(501)
	otherBacktestRunOwnerID = uint(502)
)

var backtestRunSubmittedAt = time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)

// newBacktestRunTestDatabase holds two owners, so every read can be shown to stay within one person's runs.
func newBacktestRunTestDatabase(t *testing.T) *gorm.DB {
	database := newTestDatabase(t)
	for _, ownerID := range []uint{backtestRunOwnerID, otherBacktestRunOwnerID} {
		require.NoError(t, database.WithContext(t.Context()).Create(&entities.User{
			ID: ownerID, Email: "backtester-" + string(rune('a'+ownerID-backtestRunOwnerID)) + "@example.com",
			PasswordProof: "a-proof",
		}).Error)
	}

	return database
}

func aQueuedBacktestRun(ownerID uint, minutesAfter int) entities.BacktestRun {
	requestDto := dto.BacktestRequestDto{Symbol: "BTCUSDT", AggregationInterval: "1h", Script: "the script"}

	return entities.BacktestRun{
		OwnerID:     ownerID,
		Kind:        string(vo.BacktestRunSpotScript),
		Symbol:      "BTCUSDT",
		Interval:    "1h",
		StartTime:   time.Date(2022, 10, 9, 0, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC),
		Request:     dto.BacktestRunRequestDto{SpotScript: &requestDto},
		Status:      string(vo.BacktestRunQueued),
		SubmittedAt: backtestRunSubmittedAt.Add(time.Duration(minutesAfter) * time.Minute),
	}
}

// savedBacktestRunIn stores a run already moved to the given status, as a run with a history would be.
func savedBacktestRunIn(
	t *testing.T, repository *persistence.BacktestRunRepository, backtestRun entities.BacktestRun,
	status vo.BacktestRunStatusVo, replicaName string, finishedAt *time.Time,
) entities.BacktestRun {
	backtestRun.Status = string(status)
	backtestRun.ReplicaName = replicaName
	backtestRun.FinishedAt = finishedAt
	saved, saveError := repository.Save(t.Context(), backtestRun)
	require.NoError(t, saveError)

	return saved
}

// claimOldestQueued claims as a replica does: lock the oldest queued run, move it to running, save the claim, in one transaction.
func claimOldestQueued(
	t *testing.T, database *gorm.DB, replicaName string, startedAt time.Time,
) (entities.BacktestRun, bool, error) {
	repository := persistence.NewBacktestRunRepository(database)
	claimed := entities.BacktestRun{}
	wasClaimed := false
	claimError := persistence.NewTransactionRepository(database).Atomically(t.Context(),
		func(transactionContext context.Context) error {
			queued, found, lockError := repository.LockOldestQueued(transactionContext)
			if lockError != nil || !found {
				return lockError
			}
			claimed = domains.NewBacktestRunDomain(queued).Claimed(replicaName, startedAt).ToEntity()
			wasClaimed = true

			return repository.SaveClaim(transactionContext, claimed)
		})

	return claimed, wasClaimed, claimError
}

func TestBacktestRunRepositoryKeepsTheRequestAndShowsARunOnlyToItsOwner(t *testing.T) {
	repository := persistence.NewBacktestRunRepository(newBacktestRunTestDatabase(t))
	saved, saveError := repository.Save(t.Context(), aQueuedBacktestRun(backtestRunOwnerID, 0))
	require.NoError(t, saveError)
	require.NotZero(t, saved.ID)

	testCases := []struct {
		name          string
		viewerID      uint
		expectedFound bool
	}{
		{name: "the owner finds it", viewerID: backtestRunOwnerID, expectedFound: true},
		{name: "anyone else does not", viewerID: otherBacktestRunOwnerID, expectedFound: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			found, isFound, findError := repository.FindOneOwned(t.Context(), testCase.viewerID, saved.ID)

			require.NoError(t, findError)
			assert.Equal(t, testCase.expectedFound, isFound)
			if testCase.expectedFound {
				assert.Equal(t, "queued", found.Status)
				require.NotNil(t, found.Request.SpotScript)
				assert.Equal(t, "the script", found.Request.SpotScript.Script)
				assert.Nil(t, found.TotalReturnRate)
				assert.Nil(t, found.StartedAt)
			}
		})
	}
}

func TestBacktestRunRepositoryListsNewestFirstWithTheWholeCount(t *testing.T) {
	repository := persistence.NewBacktestRunRepository(newBacktestRunTestDatabase(t))
	savedIDs := make([]uint, 0, 25)
	for minute := range 25 {
		status := vo.BacktestRunSucceeded
		if minute < 3 {
			status = vo.BacktestRunFailed
		}
		finishedAt := backtestRunSubmittedAt.Add(time.Hour)
		saved := savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, minute), status, "", &finishedAt)
		savedIDs = append(savedIDs, saved.ID)
	}
	savedBacktestRunIn(t, repository, aQueuedBacktestRun(otherBacktestRunOwnerID, 30), vo.BacktestRunFailed, "", nil)

	testCases := []struct {
		name               string
		filter             dto.BacktestRunListQueryDto
		expectedFirstID    uint
		expectedLength     int
		expectedTotalCount int64
	}{
		{
			name: "the newest twenty of twenty-five", filter: dto.BacktestRunListQueryDto{Limit: 20},
			expectedFirstID: savedIDs[24], expectedLength: 20, expectedTotalCount: 25,
		},
		{
			name: "only the failed ones", filter: dto.BacktestRunListQueryDto{Status: "failed", Limit: 20},
			expectedFirstID: savedIDs[2], expectedLength: 3, expectedTotalCount: 3,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			backtestRuns, totalCount, findError := repository.FindPageByOwner(
				t.Context(), backtestRunOwnerID, testCase.filter)

			require.NoError(t, findError)
			require.Len(t, backtestRuns, testCase.expectedLength)
			assert.Equal(t, testCase.expectedFirstID, backtestRuns[0].ID)
			assert.Equal(t, testCase.expectedTotalCount, totalCount)
			for index := 1; index < len(backtestRuns); index++ {
				assert.Greater(t, backtestRuns[index-1].ID, backtestRuns[index].ID)
			}
		})
	}
}

func TestBacktestRunRepositoryListsTheRunsOfOneScriptOrOneStrategy(t *testing.T) {
	repository := persistence.NewBacktestRunRepository(newBacktestRunTestDatabase(t))
	strategyScriptID, tradingStrategyID := uint(88), uint(7)
	ofTheScript := aQueuedBacktestRun(backtestRunOwnerID, 0)
	ofTheScript.StrategyScriptID = &strategyScriptID
	ofTheStrategy := aQueuedBacktestRun(backtestRunOwnerID, 1)
	ofTheStrategy.TradingStrategyID = &tradingStrategyID
	savedScriptRun, firstError := repository.Save(t.Context(), ofTheScript)
	require.NoError(t, firstError)
	savedStrategyRun, secondError := repository.Save(t.Context(), ofTheStrategy)
	require.NoError(t, secondError)

	byScript, scriptCount, scriptError := repository.FindPageByOwner(t.Context(), backtestRunOwnerID,
		dto.BacktestRunListQueryDto{StrategyScriptID: strategyScriptID, Limit: 20})
	byStrategy, strategyCount, strategyError := repository.FindPageByOwner(t.Context(), backtestRunOwnerID,
		dto.BacktestRunListQueryDto{TradingStrategyID: tradingStrategyID, Limit: 20})

	require.NoError(t, scriptError)
	require.NoError(t, strategyError)
	assert.Equal(t, int64(1), scriptCount)
	assert.Equal(t, savedScriptRun.ID, byScript[0].ID)
	assert.Equal(t, int64(1), strategyCount)
	assert.Equal(t, savedStrategyRun.ID, byStrategy[0].ID)
}

func TestBacktestRunRepositoryCountsOnlyQueuedAndRunningRunsOfOneOwner(t *testing.T) {
	repository := persistence.NewBacktestRunRepository(newBacktestRunTestDatabase(t))
	finishedAt := backtestRunSubmittedAt.Add(time.Hour)
	savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 0), vo.BacktestRunQueued, "", nil)
	savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 1), vo.BacktestRunRunning, "replica-a", nil)
	for minute, status := range []vo.BacktestRunStatusVo{
		vo.BacktestRunSucceeded, vo.BacktestRunFailed, vo.BacktestRunCancelled,
	} {
		savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 2+minute), status, "", &finishedAt)
	}
	savedBacktestRunIn(t, repository, aQueuedBacktestRun(otherBacktestRunOwnerID, 9), vo.BacktestRunQueued, "", nil)

	activeCount, countError := repository.CountActiveByOwner(t.Context(), backtestRunOwnerID)

	require.NoError(t, countError)
	assert.Equal(t, 2, activeCount)
}

func TestBacktestRunRepositoryClaimsTheOldestQueuedRun(t *testing.T) {
	database := newBacktestRunTestDatabase(t)
	repository := persistence.NewBacktestRunRepository(database)
	oldest, firstError := repository.Save(t.Context(), aQueuedBacktestRun(otherBacktestRunOwnerID, 0))
	require.NoError(t, firstError)
	_, secondError := repository.Save(t.Context(), aQueuedBacktestRun(backtestRunOwnerID, 1))
	require.NoError(t, secondError)
	claimedAt := backtestRunSubmittedAt.Add(time.Minute)

	claimed, wasClaimed, claimError := claimOldestQueued(t, database, "replica-a", claimedAt)

	require.NoError(t, claimError)
	require.True(t, wasClaimed)
	assert.Equal(t, oldest.ID, claimed.ID)
	stored, _, _ := repository.FindOneOwned(t.Context(), otherBacktestRunOwnerID, oldest.ID)
	assert.Equal(t, "running", stored.Status)
	assert.Equal(t, "replica-a", stored.ReplicaName)
	require.NotNil(t, stored.StartedAt)
	assert.True(t, claimedAt.Equal(*stored.StartedAt))
}

func TestBacktestRunRepositoryGivesEachQueuedRunToExactlyOneReplica(t *testing.T) {
	const queuedRunCount, replicaCount = 12, 6
	database := newBacktestRunTestDatabase(t)
	repository := persistence.NewBacktestRunRepository(database)
	for minute := range queuedRunCount {
		_, saveError := repository.Save(t.Context(), aQueuedBacktestRun(backtestRunOwnerID, minute))
		require.NoError(t, saveError)
	}

	claimsMutex := sync.Mutex{}
	claimCounts := map[uint]int{}
	claimErrors := []error{}
	starting := sync.WaitGroup{}
	starting.Add(1)
	finishing := sync.WaitGroup{}
	for replicaIndex := range replicaCount {
		finishing.Add(1)
		go func() {
			defer finishing.Done()
			starting.Wait()
			for {
				claimed, wasClaimed, claimError := claimOldestQueued(
					t, database, "replica-"+string(rune('a'+replicaIndex)), backtestRunSubmittedAt)
				claimsMutex.Lock()
				if claimError != nil {
					claimErrors = append(claimErrors, claimError)
				}
				if wasClaimed {
					claimCounts[claimed.ID]++
				}
				claimsMutex.Unlock()
				if claimError != nil || !wasClaimed {
					return
				}
			}
		}()
	}
	starting.Done()
	finishing.Wait()

	assert.Empty(t, claimErrors)
	assert.Len(t, claimCounts, queuedRunCount)
	for backtestRunID, claimCount := range claimCounts {
		assert.Equal(t, 1, claimCount, "run %d was taken %d times", backtestRunID, claimCount)
	}
}

func TestBacktestRunRepositoryFindsNothingToClaimWhenNothingIsQueued(t *testing.T) {
	database := newBacktestRunTestDatabase(t)
	repository := persistence.NewBacktestRunRepository(database)
	cancelledAt := backtestRunSubmittedAt.Add(time.Minute)
	savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 0), vo.BacktestRunRunning, "replica-a", nil)
	savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 1), vo.BacktestRunCancelled, "", &cancelledAt)

	_, wasClaimed, claimError := claimOldestQueued(t, database, "replica-b", backtestRunSubmittedAt)

	require.NoError(t, claimError)
	assert.False(t, wasClaimed)
}

func TestBacktestRunRepositoryWritesProgressOnlyWhileTheRunIsStillThisReplicasRunningOne(t *testing.T) {
	finishedAt := backtestRunSubmittedAt.Add(time.Hour)
	testCases := []struct {
		name                string
		storedStatus        vo.BacktestRunStatusVo
		writingReplicaName  string
		expectedWritten     bool
		expectedStoredCount int
	}{
		{name: "running on this replica", storedStatus: vo.BacktestRunRunning, writingReplicaName: "replica-a",
			expectedWritten: true, expectedStoredCount: 120},
		{name: "cancelled meanwhile", storedStatus: vo.BacktestRunCancelled, writingReplicaName: "replica-a",
			expectedWritten: false, expectedStoredCount: 0},
		{name: "running on another replica", storedStatus: vo.BacktestRunRunning, writingReplicaName: "replica-b",
			expectedWritten: false, expectedStoredCount: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repository := persistence.NewBacktestRunRepository(newBacktestRunTestDatabase(t))
			var storedFinishedAt *time.Time
			if testCase.storedStatus != vo.BacktestRunRunning {
				storedFinishedAt = &finishedAt
			}
			saved := savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 0),
				testCase.storedStatus, "replica-a", storedFinishedAt)
			advanced := saved
			advanced.Status = string(vo.BacktestRunRunning)
			advanced.ReplicaName = testCase.writingReplicaName
			advanced.CompletedBarCount, advanced.TotalBarCount = 120, 480

			written, writeError := repository.UpdateIfRunning(t.Context(), advanced)

			require.NoError(t, writeError)
			assert.Equal(t, testCase.expectedWritten, written)
			stored, _, _ := repository.FindOneOwned(t.Context(), backtestRunOwnerID, saved.ID)
			assert.Equal(t, string(testCase.storedStatus), stored.Status)
			assert.Equal(t, testCase.expectedStoredCount, stored.CompletedBarCount)
		})
	}
}

func TestBacktestRunRepositoryEndsARunWithItsHeadlineFigures(t *testing.T) {
	repository := persistence.NewBacktestRunRepository(newBacktestRunTestDatabase(t))
	saved := savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 0),
		vo.BacktestRunRunning, "replica-a", nil)
	finishedAt := backtestRunSubmittedAt.Add(time.Hour)
	totalReturnRate, maximumDrawdown, tradeCount := 0.255, 0.101, 4
	ended := saved
	ended.Status = string(vo.BacktestRunSucceeded)
	ended.TotalReturnRate, ended.MaximumDrawdown, ended.TradeCount = &totalReturnRate, &maximumDrawdown, &tradeCount
	ended.FinishedAt = &finishedAt

	written, writeError := repository.UpdateIfRunning(t.Context(), ended)

	require.NoError(t, writeError)
	assert.True(t, written)
	stored, _, _ := repository.FindOneOwned(t.Context(), backtestRunOwnerID, saved.ID)
	assert.Equal(t, "succeeded", stored.Status)
	require.NotNil(t, stored.TotalReturnRate)
	assert.InDelta(t, 0.255, *stored.TotalReturnRate, 1e-9)
	assert.InDelta(t, 0.101, *stored.MaximumDrawdown, 1e-9)
	assert.Equal(t, 4, *stored.TradeCount)
	require.NotNil(t, stored.FinishedAt)
	assert.True(t, finishedAt.Equal(*stored.FinishedAt))
}

func TestBacktestRunRepositoryCancelsOnlyAnActiveRunOfItsOwner(t *testing.T) {
	finishedAt := backtestRunSubmittedAt.Add(time.Hour)
	testCases := []struct {
		name            string
		storedStatus    vo.BacktestRunStatusVo
		cancellingOwner uint
		expectedWritten bool
		expectedStatus  string
	}{
		{name: "a queued run of the owner", storedStatus: vo.BacktestRunQueued, cancellingOwner: backtestRunOwnerID,
			expectedWritten: true, expectedStatus: "cancelled"},
		{name: "a running run of the owner", storedStatus: vo.BacktestRunRunning, cancellingOwner: backtestRunOwnerID,
			expectedWritten: true, expectedStatus: "cancelled"},
		{name: "a run already succeeded", storedStatus: vo.BacktestRunSucceeded, cancellingOwner: backtestRunOwnerID,
			expectedWritten: false, expectedStatus: "succeeded"},
		{name: "somebody else's queued run", storedStatus: vo.BacktestRunQueued, cancellingOwner: otherBacktestRunOwnerID,
			expectedWritten: false, expectedStatus: "queued"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repository := persistence.NewBacktestRunRepository(newBacktestRunTestDatabase(t))
			saved := savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 0),
				testCase.storedStatus, "replica-a", nil)
			cancelled := saved
			cancelled.OwnerID = testCase.cancellingOwner
			cancelled.Status = string(vo.BacktestRunCancelled)
			cancelled.FinishedAt = &finishedAt

			written, cancelError := repository.CancelIfActive(t.Context(), cancelled)

			require.NoError(t, cancelError)
			assert.Equal(t, testCase.expectedWritten, written)
			stored, _, _ := repository.FindOneOwned(t.Context(), backtestRunOwnerID, saved.ID)
			assert.Equal(t, testCase.expectedStatus, stored.Status)
		})
	}
}

func TestBacktestRunRepositoryFailsOnlyRunningRunsOfVanishedReplicas(t *testing.T) {
	repository := persistence.NewBacktestRunRepository(newBacktestRunTestDatabase(t))
	ofTheVanished := savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 0),
		vo.BacktestRunRunning, "replica-a", nil)
	ofTheLiving := savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 1),
		vo.BacktestRunRunning, "replica-b", nil)
	stillQueued := savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 2),
		vo.BacktestRunQueued, "", nil)
	sweptAt := backtestRunSubmittedAt.Add(time.Hour)

	sweptCount, sweepError := repository.FailRunningOutside(t.Context(), []string{"replica-b"}, "被重新啟動打斷", sweptAt)

	require.NoError(t, sweepError)
	assert.Equal(t, 1, sweptCount)
	vanished, _, _ := repository.FindOneOwned(t.Context(), backtestRunOwnerID, ofTheVanished.ID)
	assert.Equal(t, "failed", vanished.Status)
	assert.Equal(t, "被重新啟動打斷", vanished.FailureReason)
	require.NotNil(t, vanished.FinishedAt)
	living, _, _ := repository.FindOneOwned(t.Context(), backtestRunOwnerID, ofTheLiving.ID)
	assert.Equal(t, "running", living.Status)
	queued, _, _ := repository.FindOneOwned(t.Context(), backtestRunOwnerID, stillQueued.ID)
	assert.Equal(t, "queued", queued.Status)
}

func TestBacktestRunRepositoryDeletesRunsEndedBeforeTheCutoffWithTheirReports(t *testing.T) {
	database := newBacktestRunTestDatabase(t)
	repository := persistence.NewBacktestRunRepository(database)
	resultRepository := persistence.NewBacktestRunResultRepository(database)
	now := backtestRunSubmittedAt.AddDate(0, 0, 31)
	longAgo := now.AddDate(0, 0, -31)
	recently := now.AddDate(0, 0, -29)
	expired := savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 0),
		vo.BacktestRunSucceeded, "replica-a", &longAgo)
	require.NoError(t, resultRepository.Save(t.Context(), entities.BacktestRunResult{
		BacktestRunID: expired.ID, SpotResult: &dto.BacktestResultDto{Symbol: "BTCUSDT"},
	}))
	kept := savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 1),
		vo.BacktestRunFailed, "replica-a", &recently)
	unfinished := savedBacktestRunIn(t, repository, aQueuedBacktestRun(backtestRunOwnerID, 2),
		vo.BacktestRunQueued, "", nil)

	deletedCount, deleteError := repository.DeleteFinishedBefore(t.Context(), now.AddDate(0, 0, -30))

	require.NoError(t, deleteError)
	assert.Equal(t, 1, deletedCount)
	_, expiredFound, _ := repository.FindOneOwned(t.Context(), backtestRunOwnerID, expired.ID)
	assert.False(t, expiredFound)
	_, reportFound, _ := resultRepository.FindOne(t.Context(), expired.ID)
	assert.False(t, reportFound)
	_, keptFound, _ := repository.FindOneOwned(t.Context(), backtestRunOwnerID, kept.ID)
	assert.True(t, keptFound)
	_, unfinishedFound, _ := repository.FindOneOwned(t.Context(), backtestRunOwnerID, unfinished.ID)
	assert.True(t, unfinishedFound)
}

func TestBacktestRunResultRepositoryKeepsEachReportWhole(t *testing.T) {
	winRate := 0.5
	testCases := []struct {
		name   string
		result func(backtestRunID uint) entities.BacktestRunResult
		check  func(t *testing.T, found entities.BacktestRunResult)
	}{
		{
			name: "a spot report",
			result: func(backtestRunID uint) entities.BacktestRunResult {
				return entities.BacktestRunResult{BacktestRunID: backtestRunID, SpotResult: &dto.BacktestResultDto{
					Symbol: "BTCUSDT", UsedCandleCount: 35000,
					Summary:     dto.BacktestSummaryDto{FinalEquity: decimal.RequireFromString("12550.5"), WinRate: &winRate},
					EquityCurve: []dto.EquityPointDto{{Equity: decimal.RequireFromString("10000")}},
					InSample:    &dto.BacktestResultDto{UsedCandleCount: 28000},
					Validation:  &dto.BacktestResultDto{UsedCandleCount: 7000},
				}}
			},
			check: func(t *testing.T, found entities.BacktestRunResult) {
				require.NotNil(t, found.SpotResult)
				assert.Nil(t, found.ContractResult)
				assert.Equal(t, 35000, found.SpotResult.UsedCandleCount)
				require.NotNil(t, found.SpotResult.InSample)
				require.NotNil(t, found.SpotResult.Validation)
				assert.Equal(t, 28000, found.SpotResult.InSample.UsedCandleCount)
				assert.Equal(t, 7000, found.SpotResult.Validation.UsedCandleCount)
				assert.True(t, decimal.RequireFromString("12550.5").Equal(found.SpotResult.Summary.FinalEquity))
				assert.Len(t, found.SpotResult.EquityCurve, 1)
			},
		},
		{
			name: "a contract report",
			result: func(backtestRunID uint) entities.BacktestRunResult {
				return entities.BacktestRunResult{BacktestRunID: backtestRunID, ContractResult: &dto.ContractBacktestResultDto{
					Symbol: "BTCUSDT", TradingMode: "longShort", Leverage: decimal.NewFromInt(5),
				}}
			},
			check: func(t *testing.T, found entities.BacktestRunResult) {
				require.NotNil(t, found.ContractResult)
				assert.Nil(t, found.SpotResult)
				assert.Equal(t, "longShort", found.ContractResult.TradingMode)
				assert.True(t, decimal.NewFromInt(5).Equal(found.ContractResult.Leverage))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			database := newBacktestRunTestDatabase(t)
			saved := savedBacktestRunIn(t, persistence.NewBacktestRunRepository(database),
				aQueuedBacktestRun(backtestRunOwnerID, 0), vo.BacktestRunRunning, "replica-a", nil)
			resultRepository := persistence.NewBacktestRunResultRepository(database)
			require.NoError(t, resultRepository.Save(t.Context(), testCase.result(saved.ID)))

			found, isFound, findError := resultRepository.FindOne(t.Context(), saved.ID)

			require.NoError(t, findError)
			require.True(t, isFound)
			testCase.check(t, found)
		})
	}
}
