package job_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// startableJob is what every job offers the job manager.
type startableJob interface {
	Start(executionContext context.Context)
	Stop()
}

// seriesJobUnderTest is one of the three new jobs over the real application and
// service, with the watchlist read signalling every round that reached it.
type seriesJobUnderTest struct {
	job    startableJob
	rounds chan string
}

func watchlistSignalling(
	mockController *gomock.Controller, rounds chan string, watchlistError error,
) *mocks.MockIContractTradingSymbolRepository {
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	watched := []entities.ContractTradingSymbol{{Symbol: "BTCUSDT", IsWatched: true}}
	symbolRepository.EXPECT().FindWatched(gomock.Any()).DoAndReturn(
		func(context.Context) ([]entities.ContractTradingSymbol, error) {
			rounds <- "round"
			if watchlistError != nil {
				return nil, watchlistError
			}

			return watched, nil
		}).AnyTimes()
	symbolRepository.EXPECT().FindAll(gomock.Any()).DoAndReturn(
		func(context.Context) ([]entities.ContractTradingSymbol, error) {
			rounds <- "round"
			if watchlistError != nil {
				return nil, watchlistError
			}

			return watched, nil
		}).AnyTimes()

	return symbolRepository
}

func newFundingRateJobUnderTest(t *testing.T, venueError error, watchlistError error) seriesJobUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	rounds := make(chan string, 256)
	settlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(mockController)
	fundingRateProxy := mocks.NewMockIContractFundingRateProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	settlementRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any()).
		Return(entities.ContractFundingRateSettlement{}, false, nil).AnyTimes()
	// One settlement the rules refuse, so there is always something to write down.
	fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), gomock.Any(), gomock.Any()).Return(
		[]vo.ContractFundingRateSettlementVo{{
			Symbol: "BTCUSDT", SettlementTime: currentTime.Add(-time.Hour),
			FundingRate: decimal.RequireFromString("0.0001"),
			MarkPrice:   decimal.NewNullDecimal(decimal.Zero),
		}}, venueError).AnyTimes()
	settlementRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()

	fundingRateJob := job.NewContractFundingRateIngestionJob(
		application.NewContractFundingRateApplication(service.NewContractFundingRateService(
			settlementRepository, watchlistSignalling(mockController, rounds, watchlistError),
			fundingRateProxy, clockProxy, 1000)),
		testInterval)
	t.Cleanup(fundingRateJob.Stop)

	return seriesJobUnderTest{job: fundingRateJob, rounds: rounds}
}

func newPositionStatisticJobUnderTest(t *testing.T, venueError error) seriesJobUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	rounds := make(chan string, 256)
	statisticRepository := mocks.NewMockIContractPositionStatisticRepository(mockController)
	statisticProxy := mocks.NewMockIContractPositionStatisticProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	statisticRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any()).
		Return(entities.ContractPositionStatistic{}, false, nil).AnyTimes()
	statisticProxy.EXPECT().FetchPositionStatistics(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, venueError).AnyTimes()
	statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()

	positionStatisticJob := job.NewContractPositionStatisticIngestionJob(
		application.NewContractPositionStatisticApplication(service.NewContractPositionStatisticService(
			statisticRepository, watchlistSignalling(mockController, rounds, nil),
			statisticProxy, clockProxy, 1000)),
		testInterval)
	t.Cleanup(positionStatisticJob.Stop)

	return seriesJobUnderTest{job: positionStatisticJob, rounds: rounds}
}

func newSpecificationRefreshJobUnderTest(t *testing.T, catalogueError error) seriesJobUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	rounds := make(chan string, 256)
	lookupProxy := mocks.NewMockIContractSymbolLookupProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	lookupProxy.EXPECT().FetchTradingSpecifications(gomock.Any()).
		Return([]vo.ContractTradingSpecificationVo{}, catalogueError).AnyTimes()

	refreshJob := job.NewContractTradingSpecificationRefreshJob(
		application.NewContractTradingSymbolApplication(
			service.NewContractTradingSymbolService(
				watchlistSignalling(mockController, rounds, nil),
				mocks.NewMockIKCandleContractRepository(mockController), lookupProxy, clockProxy),
			nil, nil, nil),
		testInterval)
	t.Cleanup(refreshJob.Stop)

	return seriesJobUnderTest{job: refreshJob, rounds: rounds}
}

func everySeriesJob(t *testing.T) map[string]func() seriesJobUnderTest {
	return map[string]func() seriesJobUnderTest{
		"資金費率":   func() seriesJobUnderTest { return newFundingRateJobUnderTest(t, nil, nil) },
		"持倉統計":   func() seriesJobUnderTest { return newPositionStatisticJobUnderTest(t, nil) },
		"交易規格刷新": func() seriesJobUnderTest { return newSpecificationRefreshJobUnderTest(t, nil) },
	}
}

func TestEverySeriesJobRunsARoundOnStartAndThenEveryInterval(t *testing.T) {
	for name, build := range everySeriesJob(t) {
		t.Run(name, func(t *testing.T) {
			underTest := build()

			underTest.job.Start(t.Context())

			assert.Equal(t, "round", nextFrom(t, underTest.rounds))
			assert.Equal(t, "round", nextFrom(t, underTest.rounds))
			assert.Equal(t, "round", nextFrom(t, underTest.rounds))
		})
	}
}

func TestEverySeriesJobRunsTheStartRoundBeforeTheFirstTick(t *testing.T) {
	underTest := newFundingRateJobUnderTest(t, nil, nil)
	started := time.Now()

	underTest.job.Start(t.Context())

	require.Equal(t, "round", nextFrom(t, underTest.rounds))
	assert.Less(t, time.Since(started), testInterval, "啟動那一輪不該等第一個間隔")
}

func TestAStoppedSeriesJobRunsNoFurtherRounds(t *testing.T) {
	for name, build := range everySeriesJob(t) {
		t.Run(name, func(t *testing.T) {
			underTest := build()
			underTest.job.Start(t.Context())
			require.Equal(t, "round", nextFrom(t, underTest.rounds))

			underTest.job.Stop()
			underTest.job.Stop()
			time.Sleep(5 * testInterval)
			drainedRounds := len(underTest.rounds)
			time.Sleep(5 * testInterval)

			assert.Equal(t, drainedRounds, len(underTest.rounds), "停下來之後不該再多跑任何一輪")
		})
	}
}

func TestASeriesJobWhoseContextIsDoneRunsNoFurtherRounds(t *testing.T) {
	underTest := newPositionStatisticJobUnderTest(t, nil)
	abandonedContext, abandon := context.WithCancel(t.Context())

	underTest.job.Start(abandonedContext)
	require.Equal(t, "round", nextFrom(t, underTest.rounds))
	abandon()
	time.Sleep(5 * testInterval)
	drainedRounds := len(underTest.rounds)
	time.Sleep(5 * testInterval)

	assert.Equal(t, drainedRounds, len(underTest.rounds))
}

func TestTheSeriesJobsWriteDownWhatWentWrongWithoutStopping(t *testing.T) {
	testCases := []struct {
		name     string
		build    func(t *testing.T) seriesJobUnderTest
		recorded string
	}{
		{name: "資金費率來源不答話", build: func(t *testing.T) seriesJobUnderTest {
			return newFundingRateJobUnderTest(t, errors.New("funding venue unreachable"), nil)
		}, recorded: "contract funding rate round got no answer for BTCUSDT: funding venue unreachable"},
		{name: "資金費率有一筆不合規則", build: func(t *testing.T) seriesJobUnderTest {
			return newFundingRateJobUnderTest(t, nil, nil)
		}, recorded: "contract funding rate round skipped BTCUSDT"},
		{name: "資金費率讀不到名單", build: func(t *testing.T) seriesJobUnderTest {
			return newFundingRateJobUnderTest(t, nil, errors.New("watchlist unreadable"))
		}, recorded: "contract funding rate round did not run: watchlist unreadable"},
		{name: "持倉統計來源不答話", build: func(t *testing.T) seriesJobUnderTest {
			return newPositionStatisticJobUnderTest(t, errors.New("statistics venue unreachable"))
		}, recorded: "contract position statistic round got no answer for BTCUSDT: statistics venue unreachable"},
		{name: "交易規格來源不答話", build: func(t *testing.T) seriesJobUnderTest {
			return newSpecificationRefreshJobUnderTest(t, errors.New("catalogue unreachable"))
		}, recorded: "contract trading specification refresh did not run"},
		{name: "交易規格刷新完成", build: func(t *testing.T) seriesJobUnderTest {
			return newSpecificationRefreshJobUnderTest(t, nil)
		}, recorded: "contract trading specification refresh updated 0 contracts"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recorded := captureRecords(t)
			underTest := testCase.build(t)

			underTest.job.Start(t.Context())

			recorded.waitFor(t, testCase.recorded)
			// A second round arriving is the point: the first going wrong did not end
			// the job.
			require.Equal(t, "round", nextFrom(t, underTest.rounds))
			require.Equal(t, "round", nextFrom(t, underTest.rounds))
		})
	}
}

// fundingRateJobWithASlowRound is the funding rate job over a watchlist read that holds
// every round until it is let go, so a stop can land while a round is running.
func fundingRateJobWithASlowRound(t *testing.T) (startableJob, chan string, chan struct{}) {
	t.Helper()

	mockController := gomock.NewController(t)
	rounds := make(chan string, 256)
	release := make(chan struct{})
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	symbolRepository.EXPECT().FindWatched(gomock.Any()).DoAndReturn(
		func(context.Context) ([]entities.ContractTradingSymbol, error) {
			rounds <- "round"
			<-release

			return []entities.ContractTradingSymbol{}, nil
		}).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()

	fundingRateJob := job.NewContractFundingRateIngestionJob(
		application.NewContractFundingRateApplication(service.NewContractFundingRateService(
			mocks.NewMockIContractFundingRateSettlementRepository(mockController), symbolRepository,
			mocks.NewMockIContractFundingRateProxy(mockController), clockProxy, 1000)),
		testInterval)

	return fundingRateJob, rounds, release
}

func TestASeriesJobStoppedDuringALongRoundRunsNoRoundAfterIt(t *testing.T) {
	// While the round runs, a tick piles up behind it. When the round ends, the tick
	// and the stop are both ready and the choice between them is random — which is
	// exactly the moment a job must still not start one more round. Repeating it
	// makes landing on each side of that choice a certainty in practice.
	stoppings := []struct {
		name string
		stop func(job startableJob, abandon context.CancelFunc)
	}{
		{name: "Stop", stop: func(stoppedJob startableJob, _ context.CancelFunc) { stoppedJob.Stop() }},
		{name: "context", stop: func(_ startableJob, abandon context.CancelFunc) { abandon() }},
	}

	for _, stopping := range stoppings {
		t.Run(stopping.name, func(t *testing.T) {
			for range 20 {
				slowJob, rounds, release := fundingRateJobWithASlowRound(t)
				jobContext, abandon := context.WithCancel(t.Context())
				slowJob.Start(jobContext)
				require.Equal(t, "round", nextFrom(t, rounds))
				release <- struct{}{}
				require.Equal(t, "round", nextFrom(t, rounds))

				stopping.stop(slowJob, abandon)
				time.Sleep(3 * testInterval)
				close(release)
				time.Sleep(3 * testInterval)

				assert.Empty(t, rounds, "停下來之後不該再多跑任何一輪")
				slowJob.Stop()
				abandon()
			}
		})
	}
}
