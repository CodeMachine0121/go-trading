package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var storageUnavailable = errors.New("storage unavailable")

func TestSymbolScreeningListsWhatIsMine(t *testing.T) {
	underTest := newSymbolScreeningUnderTest(t, 0)
	underTest.profileRepository.EXPECT().FindAllByOwner(gomock.Any(), screeningAdministratorID).
		Return([]entities.SymbolScreeningProfile{aStoredScreeningProfile()}, nil)
	underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningProfileID).
		Return(aStoredScreeningProfile(), true, nil)
	underTest.runRepository.EXPECT().FindPageByOwner(gomock.Any(), screeningAdministratorID,
		dto.SymbolScreeningRunListQueryDto{Limit: 200}).Return([]entities.SymbolScreeningRun{aQueuedRun()}, int64(1), nil)
	underTest.runRepository.EXPECT().FindPageByOwner(gomock.Any(), screeningAdministratorID,
		dto.SymbolScreeningRunListQueryDto{Limit: 20}).Return(nil, int64(0), nil)

	profiles, listError := underTest.application.ListProfiles(t.Context(), screeningAdministratorID)
	profile, getError := underTest.application.GetProfile(t.Context(), screeningAdministratorID, screeningProfileID)
	cappedPage, cappedError := underTest.application.ListRuns(t.Context(), screeningAdministratorID,
		dto.SymbolScreeningRunListQueryDto{Limit: 500})
	defaultPage, defaultError := underTest.application.ListRuns(t.Context(), screeningAdministratorID,
		dto.SymbolScreeningRunListQueryDto{})

	require.NoError(t, listError)
	require.NoError(t, getError)
	require.NoError(t, cappedError)
	require.NoError(t, defaultError)
	assert.Len(t, profiles, 1)
	assert.Equal(t, "趨勢 V3", profile.Name)
	assert.Equal(t, int64(1), cappedPage.TotalCount)
	assert.Empty(t, defaultPage.Runs)
}

func TestSymbolScreeningPassesStorageFailuresOn(t *testing.T) {
	testCases := []struct {
		name string
		act  func(underTest symbolScreeningUnderTest) error
	}{
		{name: "listing profiles", act: func(underTest symbolScreeningUnderTest) error {
			underTest.profileRepository.EXPECT().FindAllByOwner(gomock.Any(), gomock.Any()).Return(nil, storageUnavailable)
			_, err := underTest.application.ListProfiles(context.Background(), screeningAdministratorID)
			return err
		}},
		{name: "reading a profile", act: func(underTest symbolScreeningUnderTest) error {
			underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(entities.SymbolScreeningProfile{}, false, storageUnavailable)
			_, err := underTest.application.GetProfile(context.Background(), screeningAdministratorID, 4)
			return err
		}},
		{name: "changing a profile", act: func(underTest symbolScreeningUnderTest) error {
			underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(entities.SymbolScreeningProfile{}, false, storageUnavailable)
			_, err := underTest.application.UpdateProfile(context.Background(), screeningAdministratorID, 4,
				aProfileWriteNaming(screeningScriptID))
			return err
		}},
		{name: "deleting a profile", act: func(underTest symbolScreeningUnderTest) error {
			underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(entities.SymbolScreeningProfile{}, false, storageUnavailable)
			return underTest.application.DeleteProfile(context.Background(), screeningAdministratorID, 4)
		}},
		{name: "counting runs before a delete", act: func(underTest symbolScreeningUnderTest) error {
			underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(aStoredScreeningProfile(), true, nil)
			underTest.runRepository.EXPECT().CountActiveByProfile(gomock.Any(), gomock.Any()).Return(0, storageUnavailable)
			return underTest.application.DeleteProfile(context.Background(), screeningAdministratorID, 4)
		}},
		{name: "saving a new profile", act: func(underTest symbolScreeningUnderTest) error {
			underTest.theScriptIsOwned()
			underTest.profileRepository.EXPECT().ExistsName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(false, nil)
			underTest.profileRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
				Return(entities.SymbolScreeningProfile{}, storageUnavailable)
			_, err := underTest.application.CreateProfile(context.Background(), screeningAdministratorID,
				aProfileWriteNaming(screeningScriptID))
			return err
		}},
		{name: "checking the name", act: func(underTest symbolScreeningUnderTest) error {
			underTest.theScriptIsOwned()
			underTest.profileRepository.EXPECT().ExistsName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(false, storageUnavailable)
			_, err := underTest.application.CreateProfile(context.Background(), screeningAdministratorID,
				aProfileWriteNaming(screeningScriptID))
			return err
		}},
		{name: "saving a changed profile", act: func(underTest symbolScreeningUnderTest) error {
			underTest.theScriptIsOwned()
			underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(aStoredScreeningProfile(), true, nil)
			underTest.profileRepository.EXPECT().ExistsName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(false, nil)
			underTest.profileRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
				Return(entities.SymbolScreeningProfile{}, storageUnavailable)
			_, err := underTest.application.UpdateProfile(context.Background(), screeningAdministratorID, 4,
				aProfileWriteNaming(screeningScriptID))
			return err
		}},
		{name: "reading the profile to submit", act: func(underTest symbolScreeningUnderTest) error {
			underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(entities.SymbolScreeningProfile{}, false, storageUnavailable)
			_, err := underTest.application.SubmitRun(context.Background(), screeningAdministratorID,
				dto.SymbolScreeningRunSubmissionDto{ProfileID: 4, SymbolList: "XRPUSDT"})
			return err
		}},
		{name: "counting my active runs", act: func(underTest symbolScreeningUnderTest) error {
			underTest.theScriptIsOwned()
			underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(aStoredScreeningProfile(), true, nil)
			underTest.profileRepository.EXPECT().ExistsName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(false, nil)
			underTest.runRepository.EXPECT().CountActiveByOwner(gomock.Any(), gomock.Any()).Return(0, storageUnavailable)
			_, err := underTest.application.SubmitRun(context.Background(), screeningAdministratorID,
				dto.SymbolScreeningRunSubmissionDto{ProfileID: 4, SymbolList: "XRPUSDT"})
			return err
		}},
		{name: "listing runs", act: func(underTest symbolScreeningUnderTest) error {
			underTest.runRepository.EXPECT().FindPageByOwner(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, int64(0), storageUnavailable)
			_, err := underTest.application.ListRuns(context.Background(), screeningAdministratorID, dto.SymbolScreeningRunListQueryDto{})
			return err
		}},
		{name: "reading a run", act: func(underTest symbolScreeningUnderTest) error {
			underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(entities.SymbolScreeningRun{}, false, storageUnavailable)
			_, err := underTest.application.GetRun(context.Background(), screeningAdministratorID, 9, 0)
			return err
		}},
		{name: "reading a run's candidates", act: func(underTest symbolScreeningUnderTest) error {
			underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(entities.SymbolScreeningRun{ID: 9, Status: "succeeded"}, true, nil)
			underTest.candidateRepository.EXPECT().FindByRun(gomock.Any(), uint(9)).Return(nil, storageUnavailable)
			_, err := underTest.application.GetRun(context.Background(), screeningAdministratorID, 9, 0)
			return err
		}},
		{name: "reading a run to cancel", act: func(underTest symbolScreeningUnderTest) error {
			underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(entities.SymbolScreeningRun{}, false, storageUnavailable)
			_, err := underTest.application.CancelRun(context.Background(), screeningAdministratorID, 9)
			return err
		}},
		{name: "writing the cancel", act: func(underTest symbolScreeningUnderTest) error {
			underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(aQueuedRun(), true, nil)
			underTest.runRepository.EXPECT().CancelIfActive(gomock.Any(), gomock.Any()).Return(false, storageUnavailable)
			_, err := underTest.application.CancelRun(context.Background(), screeningAdministratorID, 9)
			return err
		}},
		{name: "claiming the oldest queued run", act: func(underTest symbolScreeningUnderTest) error {
			underTest.runRepository.EXPECT().LockOldestQueued(gomock.Any()).
				Return(entities.SymbolScreeningRun{}, false, storageUnavailable)
			_, err := underTest.application.RunClaimedScreening(context.Background())
			return err
		}},
		{name: "clearing expired runs", act: func(underTest symbolScreeningUnderTest) error {
			underTest.runRepository.EXPECT().DeleteFinishedBefore(gomock.Any(), gomock.Any()).Return(0, storageUnavailable)
			_, _, err := underTest.application.DeleteExpired(context.Background())
			return err
		}},
		{name: "clearing expired candles", act: func(underTest symbolScreeningUnderTest) error {
			underTest.runRepository.EXPECT().DeleteFinishedBefore(gomock.Any(), gomock.Any()).Return(0, nil)
			underTest.kCandleRepository.EXPECT().DeleteFetchedBefore(gomock.Any(), gomock.Any()).Return(0, storageUnavailable)
			_, _, err := underTest.application.DeleteExpired(context.Background())
			return err
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.ErrorIs(t, testCase.act(newSymbolScreeningUnderTest(t, 0)), storageUnavailable)
		})
	}
}

func TestSymbolScreeningRefusesWhatSlipsAwayMidway(t *testing.T) {
	t.Run("a run that ended between the read and the cancel", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).Return(aQueuedRun(), true, nil)
		underTest.runRepository.EXPECT().CancelIfActive(gomock.Any(), gomock.Any()).Return(false, nil)

		_, cancelError := underTest.application.CancelRun(t.Context(), screeningAdministratorID, screeningRunID)

		assert.ErrorIs(t, cancelError, domains.ErrSymbolScreeningRunAlreadyEnded)
	})

	t.Run("a run that cannot be found to cancel", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.SymbolScreeningRun{}, false, nil)

		_, cancelError := underTest.application.CancelRun(t.Context(), screeningAdministratorID, screeningRunID)

		assert.ErrorIs(t, cancelError, domains.ErrSymbolScreeningRunNotFound)
	})

	t.Run("a profile that cannot be found to change or delete", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.SymbolScreeningProfile{}, false, nil).Times(2)

		_, updateError := underTest.application.UpdateProfile(t.Context(), screeningAdministratorID, 5,
			aProfileWriteNaming(screeningScriptID))
		deleteError := underTest.application.DeleteProfile(t.Context(), screeningAdministratorID, 5)

		assert.ErrorIs(t, updateError, domains.ErrSymbolScreeningProfileNotFound)
		assert.ErrorIs(t, deleteError, domains.ErrSymbolScreeningProfileNotFound)
	})

	t.Run("a changed profile whose figures are out of range", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(aStoredScreeningProfile(), true, nil)
		writeDto := aProfileWriteNaming(screeningScriptID)
		writeDto.AggregationInterval = "15m"

		_, updateError := underTest.application.UpdateProfile(t.Context(), screeningAdministratorID, screeningProfileID, writeDto)

		assert.ErrorIs(t, updateError, domains.ErrSymbolScreeningValidation)
	})

	t.Run("a script that no longer suits screening by the time of submission", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		spotScript := anOwnedContractSignalScript()
		spotScript.MarketDataKind = "kCandle"
		underTest.strategyScriptRepo.EXPECT().FindOne(gomock.Any(), screeningScriptID).Return(spotScript, nil)
		underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(aStoredScreeningProfile(), true, nil)

		_, submitError := underTest.application.SubmitRun(t.Context(), screeningAdministratorID,
			dto.SymbolScreeningRunSubmissionDto{ProfileID: screeningProfileID, SymbolList: "XRPUSDT"})

		assert.ErrorIs(t, submitError, domains.ErrSymbolScreeningValidation)
	})

	t.Run("nothing queued means nothing is started", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.runRepository.EXPECT().LockOldestQueued(gomock.Any()).Return(entities.SymbolScreeningRun{}, false, nil)

		started, claimError := underTest.application.RunClaimedScreening(t.Context())

		require.NoError(t, claimError)
		assert.False(t, started)
	})
}

func TestSymbolScreeningRunFailsWhenItCannotKeepItsOwnRecords(t *testing.T) {
	t.Run("its candidates cannot be read", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.runRepository.EXPECT().LockOldestQueued(gomock.Any()).Return(aQueuedRun(), true, nil)
		underTest.runRepository.EXPECT().SaveClaim(gomock.Any(), gomock.Any()).Return(nil)
		underTest.marginTierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).
			Return(nil, domains.ErrContractAccountCredentialsMissing)
		underTest.candidateRepository.EXPECT().FindByRun(gomock.Any(), screeningRunID).Return(nil, storageUnavailable)
		underTest.candidateRepository.EXPECT().CancelUnfinished(gomock.Any(), screeningRunID).Return(nil)
		underTest.runWritesSucceed()

		underTest.startsScreening(t)
		ending := underTest.endingOf(t)

		assert.Equal(t, "failed", ending.Status)
		assert.Equal(t, "storage unavailable", ending.FailureReason)
	})

	t.Run("a candidate cannot be marked as being screened", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.runRepository.EXPECT().LockOldestQueued(gomock.Any()).Return(aQueuedRun(), true, nil)
		underTest.runRepository.EXPECT().SaveClaim(gomock.Any(), gomock.Any()).Return(nil)
		underTest.marginTierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).Return(nil, errors.New("rate limited"))
		underTest.candidateRepository.EXPECT().FindByRun(gomock.Any(), screeningRunID).
			Return(screeningCandidates("LINKUSDT"), nil)
		underTest.candidateRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(storageUnavailable)
		underTest.candidateRepository.EXPECT().CancelUnfinished(gomock.Any(), screeningRunID).Return(nil)
		underTest.runWritesSucceed()

		underTest.startsScreening(t)
		ending := underTest.endingOf(t)

		assert.Equal(t, "failed", ending.Status)
	})

	t.Run("a verdict cannot be saved", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.runRepository.EXPECT().LockOldestQueued(gomock.Any()).Return(aQueuedRun(), true, nil)
		underTest.runRepository.EXPECT().SaveClaim(gomock.Any(), gomock.Any()).Return(nil)
		underTest.marginTierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).
			Return([]vo.ContractMaintenanceMarginLadderVo{{Symbol: "FOOUSDT"}}, nil)
		underTest.candidateRepository.EXPECT().FindByRun(gomock.Any(), screeningRunID).
			Return(screeningCandidates("FOOUSDT"), nil)
		gomock.InOrder(
			underTest.candidateRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil),
			underTest.candidateRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(storageUnavailable),
		)
		underTest.isNotOnTheVenue("FOOUSDT")
		underTest.candidateRepository.EXPECT().CancelUnfinished(gomock.Any(), screeningRunID).Return(nil)
		underTest.runWritesSucceed()

		underTest.startsScreening(t)

		assert.Equal(t, "failed", underTest.endingOf(t).Status)
	})

	t.Run("the progress cannot be written", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.claimsTheRun(aQueuedRun(), "FOOUSDT")
		underTest.isNotOnTheVenue("FOOUSDT")
		gomock.InOrder(
			underTest.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).Return(false, storageUnavailable),
			underTest.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, run entities.SymbolScreeningRun) (bool, error) {
					underTest.runEnded <- run
					return true, nil
				}),
		)

		underTest.startsScreening(t)

		assert.Equal(t, "failed", underTest.endingOf(t).Status)
	})
}

// prepares feeds the market data service exactly what a test names, so each of its branches can be reached.
func (underTest symbolScreeningUnderTest) prepares(
	symbol string, listing vo.ContractSymbolListingVo, stored []entities.SymbolScreeningKCandle,
	fetched []vo.ContractMarketKCandleVo, fetchError error, settlementError error,
) {
	underTest.symbolLookupProxy.EXPECT().LookUpSymbol(gomock.Any(), symbol).Return(listing, nil)
	underTest.kCandleRepository.EXPECT().FindInRange(gomock.Any(), symbol, "1d", gomock.Any(), gomock.Any()).
		Return(stored, nil).AnyTimes()
	underTest.marketDataProxy.EXPECT().FetchAggregatedKCandles(gomock.Any(), gomock.Any()).
		Return(fetched, fetchError).AnyTimes()
	underTest.kCandleRepository.EXPECT().SaveAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	underTest.settlementRepository.EXPECT().FindInRange(gomock.Any(), symbol, gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	underTest.fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), symbol, gomock.Any()).
		Return([]vo.ContractFundingRateSettlementVo{
			{Symbol: symbol, SettlementTime: screeningPeriodEnd.Add(-200 * screeningDay), FundingRate: decimal.RequireFromString("0.0001")},
			{Symbol: symbol, SettlementTime: screeningPeriodEnd.Add(-8 * time.Hour), FundingRate: decimal.Zero},
			{Symbol: symbol, SettlementTime: screeningPeriodEnd.Add(8 * time.Hour), FundingRate: decimal.Zero},
		}, settlementError).AnyTimes()
	underTest.settlementRepository.EXPECT().SaveAll(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
}

func screeningListing(symbol string) vo.ContractSymbolListingVo {
	specification := screeningSpecification
	specification.Symbol = symbol

	return vo.ContractSymbolListingVo{IsListed: true, IsKnownToVenue: true, Specification: specification}
}

func TestSymbolScreeningMarketDataBranches(t *testing.T) {
	fetchStart := screeningPeriodEnd.Add(-180 * screeningDay)
	firstHalf := []entities.SymbolScreeningKCandle{}
	for _, bar := range dailyBars("LINKUSDT", fetchStart.Add(400*screeningDay), 0.01)[:90] {
		firstHalf = append(firstHalf, entities.SymbolScreeningKCandle{
			Symbol: "LINKUSDT", Interval: "1d", OpenTime: bar.OpenTime, Open: bar.Open, High: bar.High, Low: bar.Low,
			Close: bar.Close, QuoteVolume: bar.QuoteVolume, MarkOpen: bar.MarkOpen.Decimal, MarkHigh: bar.MarkHigh.Decimal,
			MarkLow: bar.MarkLow.Decimal, MarkClose: bar.MarkClose.Decimal,
		})
	}
	withoutMark := dailyBars("LINKUSDT", fetchStart.Add(400*screeningDay), 0.01)
	withoutMark[0].MarkClose = decimal.NullDecimal{}
	noSpecification := screeningListing("LINKUSDT")
	noSpecification.Specification = vo.ContractTradingSpecificationVo{}

	testCases := []struct {
		name            string
		listing         vo.ContractSymbolListingVo
		stored          []entities.SymbolScreeningKCandle
		fetched         []vo.ContractMarketKCandleVo
		fetchError      error
		settlementError error
		expectedStatus  string
		expectedReason  string
	}{
		{name: "half kept and the rest fetched", listing: screeningListing("LINKUSDT"), stored: firstHalf,
			fetched: dailyBars("LINKUSDT", fetchStart.Add(400*screeningDay), 0.01)[90:], expectedStatus: "passed"},
		{name: "a bar without its mark price is left out", listing: screeningListing("LINKUSDT"), fetched: withoutMark,
			expectedStatus: "rejected", expectedReason: "歷史只有 179 天"},
		{name: "no specification from the venue", listing: noSpecification, expectedStatus: "unfinished",
			expectedReason: "幣安沒有提供這個合約的交易規格"},
		{name: "the candles cannot be fetched", listing: screeningListing("LINKUSDT"),
			fetchError: errors.New("venue timed out"), expectedStatus: "unfinished", expectedReason: "行情取不到"},
		{name: "the funding rates cannot be fetched", listing: screeningListing("LINKUSDT"),
			fetched: dailyBars("LINKUSDT", fetchStart.Add(400*screeningDay), 0.01), settlementError: errors.New("venue timed out"),
			expectedStatus: "unfinished", expectedReason: "資金費率取不到"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newSymbolScreeningUnderTest(t, 0)
			run := aQueuedRun()
			run.CandidateCount = 1
			underTest.claimsTheRun(run, "LINKUSDT")
			underTest.runWritesSucceed()
			underTest.scriptTradesEveryTenBars()
			underTest.prepares("LINKUSDT", testCase.listing, testCase.stored, testCase.fetched,
				testCase.fetchError, testCase.settlementError)

			underTest.startsScreening(t)
			underTest.endingOf(t)

			candidate := underTest.candidateSaved("LINKUSDT")
			assert.Equal(t, testCase.expectedStatus, candidate.Status)
			assert.Contains(t, candidate.Reason, testCase.expectedReason)
		})
	}
}

func TestSymbolScreeningBaselineThatNoLongerTrades(t *testing.T) {
	underTest := newSymbolScreeningUnderTest(t, 0)
	run := aQueuedRun("OMGUSDT")
	run.CandidateCount = 1
	underTest.claimsTheRun(run, "LINKUSDT")
	underTest.runWritesSucceed()
	underTest.isTrading("LINKUSDT", 0.01)
	underTest.scriptTradesEveryTenBars()
	underTest.symbolLookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "OMGUSDT").
		Return(vo.ContractSymbolListingVo{IsKnownToVenue: true}, nil)

	underTest.startsScreening(t)
	ending := underTest.endingOf(t)

	assert.Equal(t, "現有組合標的 OMGUSDT 無法比較：目前沒有在交易", ending.ContributionFailureReason)
}

func TestSymbolScreeningRunOnThisReplica(t *testing.T) {
	t.Run("cancelling stops it at once and writes no failure over the cancel", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		run := aQueuedRun()
		underTest.claimsTheRun(run, "LINKUSDT", "ADAUSDT")
		underTest.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).Times(0)
		waiting := underTest.blocksUntilStopped("LINKUSDT")
		underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningRunID).
			Return(entities.SymbolScreeningRun{ID: screeningRunID, OwnerID: screeningAdministratorID, Status: "running"}, true, nil)
		underTest.runRepository.EXPECT().CancelIfActive(gomock.Any(), gomock.Any()).Return(true, nil)

		underTest.startsScreening(t)
		<-waiting
		cancelled, cancelError := underTest.application.CancelRun(t.Context(), screeningAdministratorID, screeningRunID)
		underTest.application.StopRunner()

		require.NoError(t, cancelError)
		assert.Equal(t, "cancelled", cancelled.Status)
		assert.Equal(t, screeningRunID, <-underTest.unfinishedCancelled)
	})

	t.Run("a second claim waits while one run is being screened", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.claimsTheRun(aQueuedRun(), "LINKUSDT", "ADAUSDT")
		underTest.runWritesSucceed()
		waiting := underTest.blocksUntilStopped("LINKUSDT")

		underTest.startsScreening(t)
		<-waiting
		started, claimError := underTest.application.RunClaimedScreening(t.Context())
		underTest.application.StopRunner()

		require.NoError(t, claimError)
		assert.False(t, started)
	})
}

func TestSymbolScreeningRunEndsWellEvenWhenThingsGoWrong(t *testing.T) {
	t.Run("a script that panics leaves the run failed in plain words", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		run := aQueuedRun()
		run.CandidateCount = 1
		underTest.claimsTheRun(run, "LINKUSDT")
		underTest.runWritesSucceed()
		underTest.isTrading("LINKUSDT", 0.01)
		underTest.contractScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(context.Context, string, domains.IndicatorResultTypeDomain, []vo.ContractKCandleVo,
				domains.StrategyScriptParametersDomain, domaininterface.IReplayProgressSink) ([]map[string]vo.IndicatorValueVo, error) {
				panic("the compartment fell over")
			})

		underTest.startsScreening(t)

		assert.Equal(t, "篩選服務壞掉了，這筆沒有跑完", underTest.endingOf(t).FailureReason)
	})

	t.Run("time running out mid replay fails the run", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 100*time.Millisecond)
		run := aQueuedRun()
		run.CandidateCount = 1
		underTest.claimsTheRun(run, "LINKUSDT")
		underTest.runWritesSucceed()
		underTest.isTrading("LINKUSDT", 0.01)
		underTest.contractScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(executionContext context.Context, _ string, _ domains.IndicatorResultTypeDomain, _ []vo.ContractKCandleVo,
				_ domains.StrategyScriptParametersDomain, _ domaininterface.IReplayProgressSink) ([]map[string]vo.IndicatorValueVo, error) {
				<-executionContext.Done()
				return nil, executionContext.Err()
			}).AnyTimes()

		underTest.startsScreening(t)

		assert.Equal(t, "超過時間上限", underTest.endingOf(t).FailureReason)
	})

	t.Run("time running out while comparing with the baseline fails the run", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 300*time.Millisecond)
		run := aQueuedRun("BTCUSDT")
		run.CandidateCount = 1
		underTest.claimsTheRun(run, "LINKUSDT")
		underTest.runWritesSucceed()
		underTest.isTrading("LINKUSDT", 0.01)
		underTest.scriptTradesEveryTenBars()
		underTest.symbolLookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "BTCUSDT").
			DoAndReturn(func(executionContext context.Context, _ string) (vo.ContractSymbolListingVo, error) {
				<-executionContext.Done()
				return vo.ContractSymbolListingVo{}, executionContext.Err()
			})

		underTest.startsScreening(t)

		assert.Equal(t, "超過時間上限", underTest.endingOf(t).FailureReason)
	})

	t.Run("a rejected candidate is left out of the comparison", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.claimsTheRun(aQueuedRun("BTCUSDT"), "FOOUSDT", "LINKUSDT")
		underTest.runWritesSucceed()
		underTest.isNotOnTheVenue("FOOUSDT")
		underTest.isTrading("LINKUSDT", 0.01)
		underTest.isTrading("BTCUSDT", 0.01)
		underTest.scriptTradesEveryTenBars()

		underTest.startsScreening(t)
		underTest.endingOf(t)

		assert.Nil(t, underTest.candidateSaved("FOOUSDT").Findings.Contribution)
		assert.NotNil(t, underTest.candidateSaved("LINKUSDT").Findings.Contribution)
	})

	t.Run("a run cancelled just before its ending keeps the cancel", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		run := aQueuedRun()
		run.CandidateCount = 1
		underTest.claimsTheRun(run, "FOOUSDT")
		underTest.isNotOnTheVenue("FOOUSDT")
		refused := make(chan struct{})
		gomock.InOrder(
			underTest.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).Return(true, nil),
			underTest.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).
				DoAndReturn(func(context.Context, entities.SymbolScreeningRun) (bool, error) {
					close(refused)
					return false, nil
				}),
		)

		underTest.startsScreening(t)
		<-refused
		underTest.application.StopRunner()

		assert.Empty(t, underTest.unfinishedCancelled)
	})

	t.Run("an ending that cannot be written is only logged", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		run := aQueuedRun()
		run.CandidateCount = 1
		underTest.claimsTheRun(run, "FOOUSDT")
		underTest.isNotOnTheVenue("FOOUSDT")
		tried := make(chan struct{})
		gomock.InOrder(
			underTest.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).Return(true, nil),
			underTest.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).
				DoAndReturn(func(context.Context, entities.SymbolScreeningRun) (bool, error) {
					close(tried)
					return false, storageUnavailable
				}),
		)

		underTest.startsScreening(t)
		<-tried
		underTest.application.StopRunner()
	})
}

func TestSymbolScreeningMarketDataStorageFailures(t *testing.T) {
	testCases := []struct {
		name    string
		arrange func(underTest symbolScreeningUnderTest)
	}{
		{name: "kept candles cannot be read", arrange: func(underTest symbolScreeningUnderTest) {
			underTest.kCandleRepository.EXPECT().FindInRange(gomock.Any(), "LINKUSDT", "1d", gomock.Any(), gomock.Any()).
				Return(nil, storageUnavailable)
		}},
		{name: "fetched candles cannot be kept", arrange: func(underTest symbolScreeningUnderTest) {
			underTest.kCandleRepository.EXPECT().FindInRange(gomock.Any(), "LINKUSDT", "1d", gomock.Any(), gomock.Any()).
				Return(nil, nil)
			underTest.marketDataProxy.EXPECT().FetchAggregatedKCandles(gomock.Any(), gomock.Any()).Return(nil, nil)
			underTest.kCandleRepository.EXPECT().SaveAll(gomock.Any(), gomock.Any()).Return(storageUnavailable)
		}},
		{name: "kept settlements cannot be read", arrange: func(underTest symbolScreeningUnderTest) {
			underTest.kCandleRepository.EXPECT().FindInRange(gomock.Any(), "LINKUSDT", "1d", gomock.Any(), gomock.Any()).
				Return(nil, nil)
			underTest.marketDataProxy.EXPECT().FetchAggregatedKCandles(gomock.Any(), gomock.Any()).Return(nil, nil)
			underTest.kCandleRepository.EXPECT().SaveAll(gomock.Any(), gomock.Any()).Return(nil)
			underTest.settlementRepository.EXPECT().FindInRange(gomock.Any(), "LINKUSDT", gomock.Any(), gomock.Any()).
				Return(nil, storageUnavailable)
		}},
		{name: "fetched settlements cannot be kept", arrange: func(underTest symbolScreeningUnderTest) {
			underTest.kCandleRepository.EXPECT().FindInRange(gomock.Any(), "LINKUSDT", "1d", gomock.Any(), gomock.Any()).
				Return(nil, nil)
			underTest.marketDataProxy.EXPECT().FetchAggregatedKCandles(gomock.Any(), gomock.Any()).Return(nil, nil)
			underTest.kCandleRepository.EXPECT().SaveAll(gomock.Any(), gomock.Any()).Return(nil)
			underTest.settlementRepository.EXPECT().FindInRange(gomock.Any(), "LINKUSDT", gomock.Any(), gomock.Any()).
				Return(nil, nil)
			underTest.fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), "LINKUSDT", gomock.Any()).Return(nil, nil)
			underTest.settlementRepository.EXPECT().SaveAll(gomock.Any(), gomock.Any()).Return(storageUnavailable)
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newSymbolScreeningUnderTest(t, 0)
			run := aQueuedRun()
			run.CandidateCount = 1
			underTest.claimsTheRun(run, "LINKUSDT")
			underTest.runWritesSucceed()
			underTest.symbolLookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "LINKUSDT").Return(screeningListing("LINKUSDT"), nil)
			testCase.arrange(underTest)

			underTest.startsScreening(t)
			underTest.endingOf(t)

			candidate := underTest.candidateSaved("LINKUSDT")
			assert.Equal(t, "unfinished", candidate.Status)
			assert.Equal(t, "storage unavailable", candidate.Reason)
		})
	}
}

func TestSymbolScreeningReplayRefusalsLeaveTheCandidateUnfinished(t *testing.T) {
	testCases := []struct {
		name           string
		change         func(run *entities.SymbolScreeningRun, listing *vo.ContractSymbolListingVo)
		dailyChange    float64
		expectedReason string
	}{
		{name: "a specification the venue gave wrongly", change: func(_ *entities.SymbolScreeningRun, listing *vo.ContractSymbolListingVo) {
			listing.Specification.TickSize = decimal.Zero
		}, expectedReason: "價格跳動單位必須大於零"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newSymbolScreeningUnderTest(t, 0)
			run := aQueuedRun()
			run.CandidateCount = 1
			listing := screeningListing("LINKUSDT")
			testCase.change(&run, &listing)
			underTest.claimsTheRun(run, "LINKUSDT")
			underTest.runWritesSucceed()
			underTest.prepares("LINKUSDT", listing, nil, dailyBars("LINKUSDT",
				screeningPeriodEnd.Add(220*screeningDay), 0.01), nil, nil)

			underTest.startsScreening(t)
			underTest.endingOf(t)

			candidate := underTest.candidateSaved("LINKUSDT")
			assert.Equal(t, "unfinished", candidate.Status)
			assert.Contains(t, candidate.Reason, testCase.expectedReason)
		})
	}
}

func TestSymbolScreeningRefusesALeverageTheContractsLadderDoesNotAllow(t *testing.T) {
	underTest := newSymbolScreeningUnderTest(t, 0)
	run := aQueuedRun()
	run.CandidateCount = 1
	run.Snapshot.Leverage = decimal.NewFromInt(50)
	underTest.runRepository.EXPECT().LockOldestQueued(gomock.Any()).Return(run, true, nil)
	underTest.runRepository.EXPECT().SaveClaim(gomock.Any(), gomock.Any()).Return(nil)
	underTest.marginTierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).
		Return([]vo.ContractMaintenanceMarginLadderVo{{Symbol: "LINKUSDT", Tiers: []vo.ContractMaintenanceMarginTierVo{{
			Tier: 1, NotionalCap: decimal.NewFromInt(5_000_000), MaintenanceMarginRate: decimal.RequireFromString("0.01"),
			MaximumLeverage: 20,
		}}}}, nil)
	underTest.candidateRepository.EXPECT().FindByRun(gomock.Any(), screeningRunID).Return(screeningCandidates("LINKUSDT"), nil)
	underTest.candidateRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, candidate entities.SymbolScreeningCandidate) error {
			underTest.savedCandidatesMutex.Lock()
			defer underTest.savedCandidatesMutex.Unlock()
			underTest.savedCandidates[candidate.Symbol] = candidate
			return nil
		}).AnyTimes()
	underTest.runWritesSucceed()
	underTest.isTrading("LINKUSDT", 0.01)

	underTest.startsScreening(t)
	underTest.endingOf(t)

	candidate := underTest.candidateSaved("LINKUSDT")
	assert.Equal(t, "unfinished", candidate.Status)
	assert.Contains(t, candidate.Reason, "最高只能開 20 倍槓桿")
}

func TestSymbolScreeningCandidateWhoseMarketStoppedMidway(t *testing.T) {
	underTest := newSymbolScreeningUnderTest(t, 0)
	run := aQueuedRun()
	run.CandidateCount = 1
	run.Snapshot.MinimumAverageDailyQuoteVolume = decimal.Zero
	underTest.claimsTheRun(run, "LINKUSDT")
	underTest.runWritesSucceed()
	underTest.scriptTradesEveryTenBars()
	fetchStart := screeningPeriodEnd.Add(-180 * screeningDay)
	firstTwentyDays := dailyBars("LINKUSDT", fetchStart.Add(400*screeningDay), 0.01)[:20]
	underTest.prepares("LINKUSDT", screeningListing("LINKUSDT"), nil, firstTwentyDays, nil, nil)

	underTest.startsScreening(t)
	underTest.endingOf(t)

	candidate := underTest.candidateSaved("LINKUSDT")
	assert.Equal(t, "unfinished", candidate.Status)
	assert.Contains(t, candidate.Reason, "湊不出足夠的 K 線")
}

func TestSymbolScreeningRunWhoseVerdictsCannotBeWrittenAtTheEnd(t *testing.T) {
	testCases := []struct {
		name  string
		write func() error
	}{
		{name: "the store refuses", write: func() error { return storageUnavailable }},
		{name: "the store falls over", write: func() error { panic("connection reset") }},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newSymbolScreeningUnderTest(t, 0)
			run := aQueuedRun()
			run.CandidateCount = 1
			underTest.runRepository.EXPECT().LockOldestQueued(gomock.Any()).Return(run, true, nil)
			underTest.runRepository.EXPECT().SaveClaim(gomock.Any(), gomock.Any()).Return(nil)
			underTest.marginTierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).Return(nil, nil)
			underTest.candidateRepository.EXPECT().FindByRun(gomock.Any(), screeningRunID).Return(screeningCandidates("FOOUSDT"), nil)
			finalWriteTried := make(chan struct{})
			gomock.InOrder(
				underTest.candidateRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).Times(2),
				underTest.candidateRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					DoAndReturn(func(context.Context, entities.SymbolScreeningCandidate) error {
						close(finalWriteTried)
						return testCase.write()
					}),
			)
			underTest.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).Return(true, nil)
			underTest.isNotOnTheVenue("FOOUSDT")

			underTest.startsScreening(t)
			<-finalWriteTried
			underTest.application.StopRunner()
		})
	}
}

func TestSymbolScreeningReplaysWithoutPositionStatisticsAndLeavesTheFollowedListAlone(t *testing.T) {
	underTest := newSymbolScreeningUnderTest(t, 0)
	run := aQueuedRun()
	run.CandidateCount = 1
	underTest.claimsTheRun(run, "LINKUSDT")
	underTest.runWritesSucceed()
	underTest.isTrading("LINKUSDT", 0.01)
	sawOpenInterest := make(chan bool, 8)
	underTest.contractScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), "the script", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ domains.IndicatorResultTypeDomain, bars []vo.ContractKCandleVo,
			_ domains.StrategyScriptParametersDomain, _ domaininterface.IReplayProgressSink) ([]map[string]vo.IndicatorValueVo, error) {
			anyOpenInterest := false
			for _, bar := range bars {
				anyOpenInterest = anyOpenInterest || bar.OpenInterest != 0 || bar.AccountLongShortRatio != 0
			}
			sawOpenInterest <- anyOpenInterest
			signals := make([]vo.SignalVo, len(bars))
			for index := range signals {
				signals[index] = vo.SignalHold
			}
			return signalsSaying(signals...), nil
		}).Times(3)

	underTest.startsScreening(t)
	underTest.endingOf(t)

	for range 3 {
		assert.False(t, <-sawOpenInterest)
	}
	// The fixture's followed-market repositories carry no expectations, so any read or write of them fails this test.
}
