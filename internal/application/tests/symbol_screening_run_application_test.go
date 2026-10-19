package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func (underTest symbolScreeningUnderTest) isNotOnTheVenue(symbol string) {
	underTest.symbolLookupProxy.EXPECT().LookUpSymbol(gomock.Any(), symbol).Return(vo.ContractSymbolListingVo{}, nil)
}

func (underTest symbolScreeningUnderTest) startsScreening(t *testing.T) {
	started, claimError := underTest.application.RunClaimedScreening(t.Context())
	require.NoError(t, claimError)
	require.True(t, started)
}

func TestSymbolScreeningRunJudgesEachCandidate(t *testing.T) {
	t.Run("a steadily rising market passes, a symbol the venue lacks is rejected, and the passed one ranks first", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.claimsTheRun(aQueuedRun(), "FOOUSDT", "LINKUSDT")
		underTest.runWritesSucceed()
		underTest.isNotOnTheVenue("FOOUSDT")
		underTest.isTrading("LINKUSDT", 0.01)
		underTest.scriptTradesEveryTenBars()

		underTest.startsScreening(t)
		ending := underTest.endingOf(t)

		assert.Equal(t, "succeeded", ending.Status)
		assert.Equal(t, 2, ending.ProcessedCandidateCount)
		passed := underTest.candidateSaved("LINKUSDT")
		assert.Equal(t, "passed", passed.Status)
		require.NotNil(t, passed.RecommendationRank)
		assert.Equal(t, 1, *passed.RecommendationRank)
		assert.Len(t, passed.Findings.Segments, 3)
		rejected := underTest.candidateSaved("FOOUSDT")
		assert.Equal(t, "rejected", rejected.Status)
		assert.Equal(t, "basicCondition", rejected.Stage)
		assert.Equal(t, "幣安沒有這個 USDT 永續合約", rejected.Reason)
		require.NotNil(t, rejected.RecommendationRank)
		assert.Equal(t, 2, *rejected.RecommendationRank)
	})

	t.Run("every candidate rejected still completes the run", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.claimsTheRun(aQueuedRun(), "LINKUSDT", "ADAUSDT")
		underTest.runWritesSucceed()
		underTest.isTrading("LINKUSDT", -0.01)
		underTest.isTrading("ADAUSDT", -0.01)
		underTest.scriptTradesEveryTenBars()

		underTest.startsScreening(t)
		ending := underTest.endingOf(t)

		assert.Equal(t, "succeeded", ending.Status)
		for _, symbol := range []string{"LINKUSDT", "ADAUSDT"} {
			rejected := underTest.candidateSaved(symbol)
			assert.Equal(t, "rejected", rejected.Status)
			assert.Equal(t, "portability", rejected.Stage)
			assert.Contains(t, rejected.Reason, "多數區段虧損")
		}
	})

	t.Run("a candidate whose market data cannot be had is unfinished and the others go on", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.claimsTheRun(aQueuedRun(), "XRPUSDT", "LINKUSDT")
		underTest.runWritesSucceed()
		underTest.symbolLookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "XRPUSDT").
			Return(vo.ContractSymbolListingVo{}, errors.New("venue did not answer"))
		underTest.isTrading("LINKUSDT", 0.01)
		underTest.scriptTradesEveryTenBars()

		underTest.startsScreening(t)
		ending := underTest.endingOf(t)

		assert.Equal(t, "succeeded", ending.Status)
		unfinished := underTest.candidateSaved("XRPUSDT")
		assert.Equal(t, "unfinished", unfinished.Status)
		assert.Contains(t, unfinished.Reason, "行情取不到")
		assert.Equal(t, "passed", underTest.candidateSaved("LINKUSDT").Status)
	})

	t.Run("a script failing on a candidate leaves that candidate unfinished with its words", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		run := aQueuedRun()
		run.CandidateCount = 1
		underTest.claimsTheRun(run, "LINKUSDT")
		underTest.runWritesSucceed()
		underTest.isTrading("LINKUSDT", 0.01)
		underTest.contractScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("第 3 行除以零"))

		underTest.startsScreening(t)
		ending := underTest.endingOf(t)

		assert.Equal(t, "succeeded", ending.Status)
		unfinished := underTest.candidateSaved("LINKUSDT")
		assert.Equal(t, "unfinished", unfinished.Status)
		assert.Contains(t, unfinished.Reason, "第 3 行除以零")
	})
}

func TestSymbolScreeningRunComparesWithTheBaseline(t *testing.T) {
	t.Run("a passed candidate carries how it changes the baseline's longest gap", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		run := aQueuedRun("BTCUSDT")
		run.CandidateCount = 1
		underTest.claimsTheRun(run, "LINKUSDT")
		underTest.runWritesSucceed()
		underTest.isTrading("LINKUSDT", 0.01)
		underTest.isTrading("BTCUSDT", 0.01)
		underTest.scriptTradesEveryTenBars()

		underTest.startsScreening(t)
		ending := underTest.endingOf(t)

		assert.Equal(t, "succeeded", ending.Status)
		assert.Empty(t, ending.ContributionFailureReason)
		contribution := underTest.candidateSaved("LINKUSDT").Findings.Contribution
		require.NotNil(t, contribution)
		assert.Equal(t, contribution.BaselineLongestGapDays-contribution.CombinedLongestGapDays,
			contribution.GapReductionDays)
	})

	t.Run("a baseline symbol without market data leaves the contribution uncomputed but the candidate passed", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		run := aQueuedRun("BTCUSDT")
		run.CandidateCount = 1
		underTest.claimsTheRun(run, "LINKUSDT")
		underTest.runWritesSucceed()
		underTest.isTrading("LINKUSDT", 0.01)
		underTest.symbolLookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "BTCUSDT").
			Return(vo.ContractSymbolListingVo{}, errors.New("venue did not answer"))
		underTest.scriptTradesEveryTenBars()

		underTest.startsScreening(t)
		ending := underTest.endingOf(t)

		assert.Equal(t, "succeeded", ending.Status)
		assert.Contains(t, ending.ContributionFailureReason, "現有組合標的 BTCUSDT 無法比較")
		candidate := underTest.candidateSaved("LINKUSDT")
		assert.Equal(t, "passed", candidate.Status)
		assert.Nil(t, candidate.Findings.Contribution)
	})
}

// blocksUntilStopped makes the venue hang on the symbol until the run is stopped, telling the test once it is waiting.
func (underTest symbolScreeningUnderTest) blocksUntilStopped(symbol string) chan struct{} {
	waiting := make(chan struct{})
	underTest.symbolLookupProxy.EXPECT().LookUpSymbol(gomock.Any(), symbol).
		DoAndReturn(func(executionContext context.Context, _ string) (vo.ContractSymbolListingVo, error) {
			close(waiting)
			<-executionContext.Done()
			return vo.ContractSymbolListingVo{}, executionContext.Err()
		})

	return waiting
}

func TestSymbolScreeningRunStopsEarly(t *testing.T) {
	t.Run("over the time allowance it fails and keeps nothing unfinished", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 50*time.Millisecond)
		underTest.claimsTheRun(aQueuedRun(), "LINKUSDT", "ADAUSDT")
		underTest.runWritesSucceed()
		underTest.blocksUntilStopped("LINKUSDT")

		underTest.startsScreening(t)
		ending := underTest.endingOf(t)

		assert.Equal(t, "failed", ending.Status)
		assert.Equal(t, "超過時間上限", ending.FailureReason)
		assert.Equal(t, screeningRunID, <-underTest.unfinishedCancelled)
	})

	t.Run("a restart fails it as interrupted", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.claimsTheRun(aQueuedRun(), "LINKUSDT", "ADAUSDT")
		underTest.runWritesSucceed()
		waiting := underTest.blocksUntilStopped("LINKUSDT")

		underTest.startsScreening(t)
		<-waiting
		underTest.application.StopRunner()
		ending := underTest.endingOf(t)

		assert.Equal(t, "failed", ending.Status)
		assert.Equal(t, "被重新啟動打斷", ending.FailureReason)
	})

	t.Run("cancelled elsewhere, it stops at the candidate in hand and writes no ending", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.claimsTheRun(aQueuedRun(), "FOOUSDT", "LINKUSDT")
		underTest.isNotOnTheVenue("FOOUSDT")
		refused := make(chan struct{})
		gomock.InOrder(
			underTest.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).Return(true, nil),
			underTest.runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, written entities.SymbolScreeningRun) (bool, error) {
					assert.Equal(t, "running", written.Status)
					close(refused)
					return false, nil
				}),
		)

		underTest.startsScreening(t)
		<-refused
		underTest.application.StopRunner()

		assert.Empty(t, underTest.candidateSaved("LINKUSDT").Status)
		assert.Empty(t, underTest.unfinishedCancelled)
	})
}

func TestCancelSymbolScreeningRun(t *testing.T) {
	t.Run("a queued run is cancelled with its waiting candidates", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningRunID).
			Return(aQueuedRun(), true, nil)
		underTest.runRepository.EXPECT().CancelIfActive(gomock.Any(), gomock.Any()).Return(true, nil)
		underTest.candidateRepository.EXPECT().CancelUnfinished(gomock.Any(), screeningRunID).Return(nil)

		cancelled, cancelError := underTest.application.CancelRun(t.Context(), screeningAdministratorID, screeningRunID)

		require.NoError(t, cancelError)
		assert.Equal(t, "cancelled", cancelled.Status)
	})

	t.Run("an ended run cannot be cancelled", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		ended := aQueuedRun()
		ended.Status = "succeeded"
		underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningRunID).
			Return(ended, true, nil)

		_, cancelError := underTest.application.CancelRun(t.Context(), screeningAdministratorID, screeningRunID)

		assert.ErrorIs(t, cancelError, domains.ErrSymbolScreeningRunAlreadyEnded)
	})
}

func TestGetSymbolScreeningRun(t *testing.T) {
	t.Run("another administrator's run cannot be found", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, uint(70)).
			Return(entities.SymbolScreeningRun{}, false, nil)

		_, getError := underTest.application.GetRun(t.Context(), screeningAdministratorID, 70, 0)

		assert.ErrorIs(t, getError, domains.ErrSymbolScreeningRunNotFound)
	})

	t.Run("a wait answers as soon as the run ends, with candidates in recommendation order", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		running := aQueuedRun()
		running.Status = "running"
		ended := aQueuedRun()
		ended.Status = "succeeded"
		gomock.InOrder(
			underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningRunID).
				Return(running, true, nil),
			underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningRunID).
				Return(ended, true, nil),
		)
		first, second := 1, 2
		underTest.candidateRepository.EXPECT().FindByRun(gomock.Any(), screeningRunID).Return([]entities.SymbolScreeningCandidate{
			{Position: 1, Symbol: "FOOUSDT", Status: "rejected", RecommendationRank: &second},
			{Position: 2, Symbol: "LINKUSDT", Status: "passed", RecommendationRank: &first},
		}, nil)

		detail, getError := underTest.application.GetRun(t.Context(), screeningAdministratorID, screeningRunID, time.Minute)

		require.NoError(t, getError)
		assert.Equal(t, "succeeded", detail.Status)
		require.Len(t, detail.Candidates, 2)
		assert.Equal(t, "LINKUSDT", detail.Candidates[0].Symbol)
	})

	t.Run("once the server is shutting down a wait answers at once with the run as it stands", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		running := aQueuedRun()
		running.Status = "running"
		underTest.runRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningRunID).
			Return(running, true, nil).Times(1)
		underTest.candidateRepository.EXPECT().FindByRun(gomock.Any(), screeningRunID).Return(nil, nil)

		underTest.application.StopWaiting()
		detail, getError := underTest.application.GetRun(t.Context(), screeningAdministratorID, screeningRunID, time.Minute)

		require.NoError(t, getError)
		assert.Equal(t, "running", detail.Status)
	})
}

func TestSymbolScreeningClearsWhatOutlivedItsRetention(t *testing.T) {
	underTest := newSymbolScreeningUnderTest(t, 0)
	underTest.runRepository.EXPECT().DeleteFinishedBefore(gomock.Any(), screeningMoment.Add(-90*screeningDay)).Return(2, nil)
	underTest.kCandleRepository.EXPECT().DeleteFetchedBefore(gomock.Any(), screeningMoment.Add(-30*screeningDay)).Return(10, nil)
	underTest.settlementRepository.EXPECT().DeleteFetchedBefore(gomock.Any(), screeningMoment.Add(-30*screeningDay)).Return(3, nil)

	deletedRuns, deletedMarketData, deleteError := underTest.application.DeleteExpired(t.Context())

	require.NoError(t, deleteError)
	assert.Equal(t, 2, deletedRuns)
	assert.Equal(t, 13, deletedMarketData)
}

func TestSymbolScreeningReusesMarketDataStillKept(t *testing.T) {
	fetchStart := screeningPeriodEnd.Add(-180 * screeningDay)
	settlementStart := fetchStart.Add(-24 * time.Hour)

	testCases := []struct {
		name                string
		keptSettlementTimes []time.Time
		expectedFetchAfter  time.Time
	}{
		{name: "kept funding rates reaching back to the lead-in are reused",
			keptSettlementTimes: []time.Time{settlementStart, screeningPeriodEnd.Add(-4 * time.Hour)}},
		{name: "kept funding rates missing the start of the period are asked for again from the lead-in",
			keptSettlementTimes: []time.Time{screeningPeriodEnd.Add(-4 * time.Hour)}, expectedFetchAfter: settlementStart},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newSymbolScreeningUnderTest(t, 0)
			run := aQueuedRun()
			run.CandidateCount = 1
			underTest.claimsTheRun(run, "LINKUSDT")
			underTest.runWritesSucceed()
			underTest.scriptTradesEveryTenBars()
			specification := screeningSpecification
			underTest.symbolLookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "LINKUSDT").
				Return(vo.ContractSymbolListingVo{IsListed: true, IsKnownToVenue: true, Specification: specification}, nil)
			kept := []entities.SymbolScreeningKCandle{}
			for _, bar := range dailyBars("LINKUSDT", fetchStart.Add(400*screeningDay), 0.01) {
				kept = append(kept, entities.SymbolScreeningKCandle{
					Symbol: "LINKUSDT", Interval: "1d", OpenTime: bar.OpenTime, Open: bar.Open, High: bar.High, Low: bar.Low,
					Close: bar.Close, QuoteVolume: bar.QuoteVolume, MarkOpen: bar.MarkOpen.Decimal, MarkHigh: bar.MarkHigh.Decimal,
					MarkLow: bar.MarkLow.Decimal, MarkClose: bar.MarkClose.Decimal,
				})
			}
			underTest.kCandleRepository.EXPECT().FindInRange(gomock.Any(), "LINKUSDT", "1d", fetchStart, screeningPeriodEnd).
				Return(kept, nil)
			keptSettlements := []entities.SymbolScreeningFundingRateSettlement{}
			for _, settlementTime := range testCase.keptSettlementTimes {
				keptSettlements = append(keptSettlements, entities.SymbolScreeningFundingRateSettlement{
					Symbol: "LINKUSDT", SettlementTime: settlementTime, FundingRate: decimal.Zero,
				})
			}
			underTest.settlementRepository.EXPECT().
				FindInRange(gomock.Any(), "LINKUSDT", settlementStart, screeningPeriodEnd).Return(keptSettlements, nil)
			if !testCase.expectedFetchAfter.IsZero() {
				underTest.fundingRateProxy.EXPECT().
					FetchFundingRateSettlements(gomock.Any(), "LINKUSDT", testCase.expectedFetchAfter).Return(nil, nil)
				underTest.settlementRepository.EXPECT().SaveAll(gomock.Any(), gomock.Any()).Return(nil)
			}

			underTest.startsScreening(t)
			ending := underTest.endingOf(t)

			assert.Equal(t, "succeeded", ending.Status)
			assert.Equal(t, "passed", underTest.candidateSaved("LINKUSDT").Status)
		})
	}
}
