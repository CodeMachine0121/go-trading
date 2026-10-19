package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	screeningAdministratorID      = uint(601)
	otherScreeningAdministratorID = uint(602)
)

var screeningSubmittedAt = time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)

func newScreeningTestDatabase(t *testing.T) *gorm.DB {
	database := newTestDatabase(t)
	for index, ownerID := range []uint{screeningAdministratorID, otherScreeningAdministratorID} {
		require.NoError(t, database.WithContext(t.Context()).Create(&entities.User{
			ID: ownerID, Email: "screener-" + string(rune('a'+index)) + "@example.com", PasswordProof: "a-proof",
		}).Error)
	}

	return database
}

func aScreeningProfileFor(ownerID uint, name string) entities.SymbolScreeningProfile {
	return entities.SymbolScreeningProfile{
		OwnerID: ownerID, Name: name, StrategyScriptID: 108, ParameterValues: []dto.StrategyScriptParameterValueDto{},
		AggregationInterval: "4h", TradingMode: "longOnly", Leverage: decimal.NewFromInt(1),
		EntryCostPercentage: decimal.RequireFromString("0.05"), SlippagePercentage: decimal.RequireFromString("0.05"),
		HistoryDays: 730, SegmentCount: 3, MinimumTradeCount: 6, MinimumAverageDailyQuoteVolume: decimal.NewFromInt(20_000_000),
		BaselineSymbols: []string{"BTCUSDT"},
	}
}

func aQueuedScreeningRun(ownerID uint, profileID *uint, symbols ...string) entities.SymbolScreeningRun {
	candidates := []entities.SymbolScreeningCandidate{}
	for index, symbol := range symbols {
		candidates = append(candidates, entities.SymbolScreeningCandidate{Position: index + 1, Symbol: symbol, Status: "waiting"})
	}

	return entities.SymbolScreeningRun{
		OwnerID: ownerID, ProfileID: profileID, ProfileName: "趨勢 V3", Status: "queued", CandidateCount: len(symbols),
		Snapshot: dto.SymbolScreeningSnapshotDto{Script: "the script", HistoryDays: 730}, SubmittedAt: screeningSubmittedAt,
		Candidates: candidates,
	}
}

func TestSymbolScreeningProfileRepositoryKeepsProfilesToTheirOwner(t *testing.T) {
	database := newScreeningTestDatabase(t)
	repository := persistence.NewSymbolScreeningProfileRepository(database)
	saved, saveError := repository.Save(t.Context(), aScreeningProfileFor(screeningAdministratorID, "趨勢 V3"))
	require.NoError(t, saveError)

	mine, foundMine, findError := repository.FindOneOwned(t.Context(), screeningAdministratorID, saved.ID)
	_, foundByOther, otherFindError := repository.FindOneOwned(t.Context(), otherScreeningAdministratorID, saved.ID)
	nameTaken, nameError := repository.ExistsName(t.Context(), screeningAdministratorID, "趨勢 V3", 0)
	ownNameKept, ownNameError := repository.ExistsName(t.Context(), screeningAdministratorID, "趨勢 V3", saved.ID)

	require.NoError(t, findError)
	require.NoError(t, otherFindError)
	require.NoError(t, nameError)
	require.NoError(t, ownNameError)
	assert.True(t, foundMine)
	assert.Equal(t, []string{"BTCUSDT"}, mine.BaselineSymbols)
	assert.False(t, foundByOther)
	assert.True(t, nameTaken)
	assert.False(t, ownNameKept)

	require.NoError(t, repository.DeleteOwned(t.Context(), otherScreeningAdministratorID, saved.ID))
	_, stillThere, _ := repository.FindOneOwned(t.Context(), screeningAdministratorID, saved.ID)
	assert.True(t, stillThere)
	require.NoError(t, repository.DeleteOwned(t.Context(), screeningAdministratorID, saved.ID))
	_, gone, _ := repository.FindOneOwned(t.Context(), screeningAdministratorID, saved.ID)
	assert.False(t, gone)
}

func TestSymbolScreeningRunRepositoryQueuesClaimsAndGuardsRuns(t *testing.T) {
	database := newScreeningTestDatabase(t)
	profileRepository := persistence.NewSymbolScreeningProfileRepository(database)
	runRepository := persistence.NewSymbolScreeningRunRepository(database)
	candidateRepository := persistence.NewSymbolScreeningCandidateRepository(database)
	transactionRepository := persistence.NewTransactionRepository(database)
	profile, profileError := profileRepository.Save(t.Context(), aScreeningProfileFor(screeningAdministratorID, "趨勢 V3"))
	require.NoError(t, profileError)

	saved, saveError := runRepository.Save(t.Context(), aQueuedScreeningRun(screeningAdministratorID, &profile.ID, "XRPUSDT", "LINKUSDT"))
	require.NoError(t, saveError)

	activeCount, countError := runRepository.CountActiveByOwner(t.Context(), screeningAdministratorID)
	usingProfile, profileCountError := runRepository.CountActiveByProfile(t.Context(), profile.ID)
	candidates, candidatesError := candidateRepository.FindByRun(t.Context(), saved.ID)
	require.NoError(t, countError)
	require.NoError(t, profileCountError)
	require.NoError(t, candidatesError)
	assert.Equal(t, 1, activeCount)
	assert.Equal(t, 1, usingProfile)
	require.Len(t, candidates, 2)
	assert.Equal(t, "XRPUSDT", candidates[0].Symbol)

	claimed := entities.SymbolScreeningRun{}
	require.NoError(t, transactionRepository.Atomically(t.Context(), func(transactionContext context.Context) error {
		queued, found, lockError := runRepository.LockOldestQueued(transactionContext)
		require.True(t, found)
		startedAt := screeningSubmittedAt.Add(time.Minute)
		queued.Status, queued.ReplicaName, queued.StartedAt = "running", "replica-a", &startedAt
		claimed = queued
		if lockError != nil {
			return lockError
		}

		return runRepository.SaveClaim(transactionContext, queued)
	}))

	claimed.ProcessedCandidateCount = 1
	advancedHere, advanceError := runRepository.UpdateIfRunning(t.Context(), claimed)
	elsewhere := claimed
	elsewhere.ReplicaName = "replica-b"
	advancedElsewhere, elsewhereError := runRepository.UpdateIfRunning(t.Context(), elsewhere)
	require.NoError(t, advanceError)
	require.NoError(t, elsewhereError)
	assert.True(t, advancedHere)
	assert.False(t, advancedElsewhere)

	require.NoError(t, candidateRepository.CancelUnfinished(t.Context(), saved.ID))
	afterCancel, _ := candidateRepository.FindByRun(t.Context(), saved.ID)
	assert.Equal(t, "cancelled", afterCancel[0].Status)

	swept, sweepError := runRepository.FailRunningOutside(t.Context(), []string{"replica-b"}, "被重新啟動打斷", screeningSubmittedAt)
	require.NoError(t, sweepError)
	assert.Equal(t, 1, swept)
	ended, _, _ := runRepository.FindOneOwned(t.Context(), screeningAdministratorID, saved.ID)
	assert.Equal(t, "failed", ended.Status)
	assert.Equal(t, "被重新啟動打斷", ended.FailureReason)

	deleted, deleteError := runRepository.DeleteFinishedBefore(t.Context(), screeningSubmittedAt.Add(time.Hour))
	require.NoError(t, deleteError)
	assert.Equal(t, 1, deleted)
	leftCandidates, _ := candidateRepository.FindByRun(t.Context(), saved.ID)
	assert.Empty(t, leftCandidates)
}

func TestSymbolScreeningRunRepositoryCancelsOnlyActiveRunsOfTheOwner(t *testing.T) {
	database := newScreeningTestDatabase(t)
	runRepository := persistence.NewSymbolScreeningRunRepository(database)
	saved, saveError := runRepository.Save(t.Context(), aQueuedScreeningRun(screeningAdministratorID, nil, "XRPUSDT"))
	require.NoError(t, saveError)
	cancelledAt := screeningSubmittedAt.Add(time.Minute)

	foreign := saved
	foreign.OwnerID, foreign.Status, foreign.FinishedAt = otherScreeningAdministratorID, "cancelled", &cancelledAt
	byOther, otherError := runRepository.CancelIfActive(t.Context(), foreign)
	mine := saved
	mine.Status, mine.FinishedAt = "cancelled", &cancelledAt
	byOwner, ownerError := runRepository.CancelIfActive(t.Context(), mine)
	again, againError := runRepository.CancelIfActive(t.Context(), mine)

	require.NoError(t, otherError)
	require.NoError(t, ownerError)
	require.NoError(t, againError)
	assert.False(t, byOther)
	assert.True(t, byOwner)
	assert.False(t, again)
}

func TestSymbolScreeningMarketDataIsReplacedAndExpires(t *testing.T) {
	database := newScreeningTestDatabase(t)
	kCandleRepository := persistence.NewSymbolScreeningKCandleRepository(database)
	settlementRepository := persistence.NewSymbolScreeningFundingRateSettlementRepository(database)
	openTime := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	aKCandle := func(closePrice int64, fetchedAt time.Time) entities.SymbolScreeningKCandle {
		price := decimal.NewFromInt(closePrice)
		return entities.SymbolScreeningKCandle{
			Symbol: "LINKUSDT", Interval: "4h", OpenTime: openTime, Open: price, High: price, Low: price, Close: price,
			MarkOpen: price, MarkHigh: price, MarkLow: price, MarkClose: price, FetchedAt: fetchedAt,
		}
	}

	require.NoError(t, kCandleRepository.SaveAll(t.Context(), []entities.SymbolScreeningKCandle{aKCandle(10, screeningSubmittedAt)}))
	require.NoError(t, kCandleRepository.SaveAll(t.Context(), []entities.SymbolScreeningKCandle{
		aKCandle(12, screeningSubmittedAt.Add(time.Hour)),
	}))
	kept, findError := kCandleRepository.FindInRange(t.Context(), "LINKUSDT", "4h", openTime, openTime.Add(4*time.Hour))
	require.NoError(t, findError)
	require.Len(t, kept, 1)
	assert.True(t, decimal.NewFromInt(12).Equal(kept[0].Close))
	excluded, _ := kCandleRepository.FindInRange(t.Context(), "LINKUSDT", "4h", openTime.Add(time.Minute), openTime.Add(4*time.Hour))
	assert.Empty(t, excluded)

	require.NoError(t, settlementRepository.SaveAll(t.Context(), []entities.SymbolScreeningFundingRateSettlement{{
		Symbol: "LINKUSDT", SettlementTime: openTime, FundingRate: decimal.RequireFromString("0.0001"), FetchedAt: screeningSubmittedAt,
	}}))
	settlements, settlementError := settlementRepository.FindInRange(t.Context(), "LINKUSDT", openTime, openTime.Add(time.Hour))
	require.NoError(t, settlementError)
	assert.Len(t, settlements, 1)

	deletedKCandles, deleteKCandlesError := kCandleRepository.DeleteFetchedBefore(t.Context(), screeningSubmittedAt.Add(2*time.Hour))
	deletedSettlements, deleteSettlementsError := settlementRepository.DeleteFetchedBefore(t.Context(), screeningSubmittedAt.Add(time.Minute))
	require.NoError(t, deleteKCandlesError)
	require.NoError(t, deleteSettlementsError)
	assert.Equal(t, 1, deletedKCandles)
	assert.Equal(t, 1, deletedSettlements)
}

func TestAUserIsNotAnAdministratorUntilTheOperatorSaysSo(t *testing.T) {
	database := newScreeningTestDatabase(t)

	stored, findError := persistence.NewUserRepository(database).FindOne(t.Context(), screeningAdministratorID)

	require.NoError(t, findError)
	assert.False(t, stored.IsAdmin)
}

func TestSymbolScreeningRepositoriesListWhatIsMine(t *testing.T) {
	database := newScreeningTestDatabase(t)
	profileRepository := persistence.NewSymbolScreeningProfileRepository(database)
	runRepository := persistence.NewSymbolScreeningRunRepository(database)
	candidateRepository := persistence.NewSymbolScreeningCandidateRepository(database)
	_, firstError := profileRepository.Save(t.Context(), aScreeningProfileFor(screeningAdministratorID, "甲"))
	_, secondError := profileRepository.Save(t.Context(), aScreeningProfileFor(screeningAdministratorID, "乙"))
	_, foreignError := profileRepository.Save(t.Context(), aScreeningProfileFor(otherScreeningAdministratorID, "丙"))
	require.NoError(t, errors.Join(firstError, secondError, foreignError))
	older, olderError := runRepository.Save(t.Context(), aQueuedScreeningRun(screeningAdministratorID, nil, "XRPUSDT"))
	newer, newerError := runRepository.Save(t.Context(), aQueuedScreeningRun(screeningAdministratorID, nil, "LINKUSDT"))
	require.NoError(t, errors.Join(olderError, newerError))
	finishedAt := screeningSubmittedAt.Add(time.Hour)
	older.Status, older.FinishedAt = "cancelled", &finishedAt
	_, cancelError := runRepository.CancelIfActive(t.Context(), older)
	require.NoError(t, cancelError)

	profiles, listError := profileRepository.FindAllByOwner(t.Context(), screeningAdministratorID)
	allRuns, allCount, pageError := runRepository.FindPageByOwner(t.Context(), screeningAdministratorID,
		dto.SymbolScreeningRunListQueryDto{Limit: 20})
	queuedRuns, queuedCount, queuedError := runRepository.FindPageByOwner(t.Context(), screeningAdministratorID,
		dto.SymbolScreeningRunListQueryDto{Status: "queued", Limit: 20})

	require.NoError(t, errors.Join(listError, pageError, queuedError))
	require.Len(t, profiles, 2)
	assert.Equal(t, "甲", profiles[0].Name)
	assert.Equal(t, int64(2), allCount)
	assert.Equal(t, newer.ID, allRuns[0].ID)
	assert.Equal(t, int64(1), queuedCount)
	assert.Equal(t, newer.ID, queuedRuns[0].ID)

	candidates, _ := candidateRepository.FindByRun(t.Context(), newer.ID)
	rank := 1
	candidates[0].Status, candidates[0].RecommendationRank = "passed", &rank
	candidates[0].Findings = dto.SymbolScreeningFindingsDto{Segments: []dto.SymbolScreeningSegmentFindingDto{{ReturnRate: 0.2}}}
	require.NoError(t, candidateRepository.Save(t.Context(), candidates[0]))
	saved, _ := candidateRepository.FindByRun(t.Context(), newer.ID)
	assert.Equal(t, "passed", saved[0].Status)
	require.NotNil(t, saved[0].RecommendationRank)
	assert.InDelta(t, 0.2, saved[0].Findings.Segments[0].ReturnRate, 0.0001)
}

func TestSymbolScreeningRepositoriesSaySoWhenTheStoreIsGone(t *testing.T) {
	database := newScreeningTestDatabase(t)
	connection, connectionError := database.DB()
	require.NoError(t, connectionError)
	require.NoError(t, connection.Close())
	profileRepository := persistence.NewSymbolScreeningProfileRepository(database)
	runRepository := persistence.NewSymbolScreeningRunRepository(database)
	candidateRepository := persistence.NewSymbolScreeningCandidateRepository(database)
	kCandleRepository := persistence.NewSymbolScreeningKCandleRepository(database)
	settlementRepository := persistence.NewSymbolScreeningFundingRateSettlementRepository(database)
	ctx := t.Context()
	someRun := entities.SymbolScreeningRun{ID: 1, OwnerID: screeningAdministratorID}

	failures := map[string]func() error{
		"save profile": func() error {
			_, err := profileRepository.Save(ctx, aScreeningProfileFor(screeningAdministratorID, "x"))
			return err
		},
		"find profile":   func() error { _, _, err := profileRepository.FindOneOwned(ctx, 1, 1); return err },
		"list profiles":  func() error { _, err := profileRepository.FindAllByOwner(ctx, 1); return err },
		"name check":     func() error { _, err := profileRepository.ExistsName(ctx, 1, "x", 0); return err },
		"delete profile": func() error { return profileRepository.DeleteOwned(ctx, 1, 1) },
		"save run":       func() error { _, err := runRepository.Save(ctx, aQueuedScreeningRun(1, nil, "X")); return err },
		"find run":       func() error { _, _, err := runRepository.FindOneOwned(ctx, 1, 1); return err },
		"list runs": func() error {
			_, _, err := runRepository.FindPageByOwner(ctx, 1, dto.SymbolScreeningRunListQueryDto{Limit: 1})
			return err
		},
		"count by owner":    func() error { _, err := runRepository.CountActiveByOwner(ctx, 1); return err },
		"count by profile":  func() error { _, err := runRepository.CountActiveByProfile(ctx, 1); return err },
		"lock oldest":       func() error { _, _, err := runRepository.LockOldestQueued(ctx); return err },
		"save claim":        func() error { return runRepository.SaveClaim(ctx, someRun) },
		"update if running": func() error { _, err := runRepository.UpdateIfRunning(ctx, someRun); return err },
		"cancel if active":  func() error { _, err := runRepository.CancelIfActive(ctx, someRun); return err },
		"fail outside": func() error {
			_, err := runRepository.FailRunningOutside(ctx, []string{"a"}, "x", time.Now())
			return err
		},
		"delete runs":       func() error { _, err := runRepository.DeleteFinishedBefore(ctx, time.Now()); return err },
		"list candidates":   func() error { _, err := candidateRepository.FindByRun(ctx, 1); return err },
		"save candidate":    func() error { return candidateRepository.Save(ctx, entities.SymbolScreeningCandidate{ID: 1}) },
		"cancel candidates": func() error { return candidateRepository.CancelUnfinished(ctx, 1) },
		"find candles": func() error {
			_, err := kCandleRepository.FindInRange(ctx, "X", "4h", time.Now(), time.Now())
			return err
		},
		"save candles":     func() error { return kCandleRepository.SaveAll(ctx, []entities.SymbolScreeningKCandle{{Symbol: "X"}}) },
		"delete candles":   func() error { _, err := kCandleRepository.DeleteFetchedBefore(ctx, time.Now()); return err },
		"find settlements": func() error { _, err := settlementRepository.FindInRange(ctx, "X", time.Now(), time.Now()); return err },
		"save settlements": func() error {
			return settlementRepository.SaveAll(ctx, []entities.SymbolScreeningFundingRateSettlement{{Symbol: "X"}})
		},
		"delete settlements": func() error { _, err := settlementRepository.DeleteFetchedBefore(ctx, time.Now()); return err },
	}

	for name, failure := range failures {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, failure())
		})
	}
	assert.NoError(t, kCandleRepository.SaveAll(ctx, nil))
	assert.NoError(t, settlementRepository.SaveAll(ctx, nil))
}

func TestSymbolScreeningRunRepositoryWithNothingThere(t *testing.T) {
	database := newScreeningTestDatabase(t)
	runRepository := persistence.NewSymbolScreeningRunRepository(database)
	transactionRepository := persistence.NewTransactionRepository(database)

	_, found, findError := runRepository.FindOneOwned(t.Context(), screeningAdministratorID, 999999)
	queuedFound := true
	lockError := transactionRepository.Atomically(t.Context(), func(transactionContext context.Context) error {
		_, isQueued, err := runRepository.LockOldestQueued(transactionContext)
		queuedFound = isQueued
		return err
	})

	require.NoError(t, errors.Join(findError, lockError))
	assert.False(t, found)
	assert.False(t, queuedFound)
}

func TestADeletedProfileLeavesItsEndedRunsWithTheSettingsTheyUsed(t *testing.T) {
	database := newScreeningTestDatabase(t)
	profileRepository := persistence.NewSymbolScreeningProfileRepository(database)
	runRepository := persistence.NewSymbolScreeningRunRepository(database)
	profile, profileError := profileRepository.Save(t.Context(), aScreeningProfileFor(screeningAdministratorID, "趨勢 V3"))
	require.NoError(t, profileError)
	run, runError := runRepository.Save(t.Context(), aQueuedScreeningRun(screeningAdministratorID, &profile.ID, "XRPUSDT"))
	require.NoError(t, runError)

	require.NoError(t, profileRepository.DeleteOwned(t.Context(), screeningAdministratorID, profile.ID))

	kept, found, findError := runRepository.FindOneOwned(t.Context(), screeningAdministratorID, run.ID)
	require.NoError(t, findError)
	require.True(t, found)
	assert.Nil(t, kept.ProfileID)
	assert.Equal(t, "趨勢 V3", kept.ProfileName)
	assert.Equal(t, 730, kept.Snapshot.HistoryDays)
}

func TestCancellingKeepsTheCandidatesAlreadyJudged(t *testing.T) {
	database := newScreeningTestDatabase(t)
	runRepository := persistence.NewSymbolScreeningRunRepository(database)
	candidateRepository := persistence.NewSymbolScreeningCandidateRepository(database)
	run, runError := runRepository.Save(t.Context(), aQueuedScreeningRun(screeningAdministratorID, nil, "FOOUSDT", "LINKUSDT", "ADAUSDT"))
	require.NoError(t, runError)
	candidates, _ := candidateRepository.FindByRun(t.Context(), run.ID)
	candidates[0].Status, candidates[0].Stage, candidates[0].Reason = "rejected", "basicCondition", "幣安沒有這個 USDT 永續合約"
	candidates[1].Status = "processing"
	require.NoError(t, errors.Join(
		candidateRepository.Save(t.Context(), candidates[0]), candidateRepository.Save(t.Context(), candidates[1])))

	require.NoError(t, candidateRepository.CancelUnfinished(t.Context(), run.ID))

	after, _ := candidateRepository.FindByRun(t.Context(), run.ID)
	assert.Equal(t, "rejected", after[0].Status)
	assert.Equal(t, "幣安沒有這個 USDT 永續合約", after[0].Reason)
	assert.Equal(t, "cancelled", after[1].Status)
	assert.Equal(t, "cancelled", after[2].Status)
}
