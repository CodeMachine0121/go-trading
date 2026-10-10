package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type backtestRunWorkersUnderTest struct {
	backtestRunApplication *application.BacktestRunApplication
	runRepository          *mocks.MockIBacktestRunRepository
	// finishOne lets exactly one replay in flight end.
	finishOne chan struct{}
}

// newBacktestRunWorkersUnderTest replays runs whose scripts each wait until told to finish.
func newBacktestRunWorkersUnderTest(t *testing.T, workerCount int) backtestRunWorkersUnderTest {
	controller := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(backtestNow).AnyTimes()
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{storedHourlyCandle(0, "100"), storedHourlyCandle(1, "110")}, nil).AnyTimes()
	finishOne := make(chan struct{})
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			executionContext context.Context, _ string, _ domains.IndicatorResultTypeDomain,
			_ []vo.KCandleVo, _ domains.StrategyScriptParametersDomain, _ domaininterface.IReplayProgressSink,
		) ([]map[string]vo.IndicatorValueVo, error) {
			select {
			case <-finishOne:
				return nil, errors.New("算式執行失敗")
			case <-executionContext.Done():
				return nil, executionContext.Err()
			}
		}).AnyTimes()
	runRepository := mocks.NewMockIBacktestRunRepository(controller)
	runRepository.EXPECT().UpdateIfRunning(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	transactionRepository := mocks.NewMockITransactionRepository(controller)
	transactionRepository.EXPECT().Atomically(gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, work func(context.Context) error) error {
			return work(executionContext)
		}).AnyTimes()
	backtestService := service.NewBacktestService(kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults, 0)

	return backtestRunWorkersUnderTest{
		backtestRunApplication: application.NewBacktestRunApplication(service.NewBacktestRunService(
			runRepository, mocks.NewMockIBacktestRunResultRepository(controller), transactionRepository, clockProxy,
			backtestService, nil, domains.NewBacktestRunAdmissionDomain(3), "replica-a", time.Minute, 100),
			workerCount, 30*24*time.Hour),
		runRepository: runRepository,
		finishOne:     finishOne,
	}
}

// queueHoldsRuns hands out this many queued runs, oldest first, then reports the queue empty.
func (underTest backtestRunWorkersUnderTest) queueHoldsRuns(runCount int) {
	nextID := uint(0)
	underTest.runRepository.EXPECT().ClaimOldestQueued(gomock.Any(), "replica-a", gomock.Any()).
		DoAndReturn(func(context.Context, string, time.Time) (entities.BacktestRun, bool, error) {
			if int(nextID) == runCount {
				return entities.BacktestRun{}, false, nil
			}
			nextID++
			requestDto := replayRequestDto()
			return entities.BacktestRun{
				ID: nextID, Kind: string(vo.BacktestRunSpotScript), Status: string(vo.BacktestRunRunning),
				ReplicaName: "replica-a", Request: dto.BacktestRunRequestDto{SpotScript: &requestDto},
			}, true, nil
		}).AnyTimes()
}

func TestRunClaimedBacktestsFillsOnlyTheFreeWorkerPlaces(t *testing.T) {
	underTest := newBacktestRunWorkersUnderTest(t, 2)
	underTest.queueHoldsRuns(3)
	t.Cleanup(underTest.backtestRunApplication.StopRunners)

	firstRound, firstError := underTest.backtestRunApplication.RunClaimedBacktests(t.Context())
	secondRound, secondError := underTest.backtestRunApplication.RunClaimedBacktests(t.Context())

	require.NoError(t, firstError)
	require.NoError(t, secondError)
	assert.Equal(t, 2, firstRound)
	assert.Equal(t, 0, secondRound, "the third run waits while both places are taken")

	underTest.finishOne <- struct{}{}
	assert.Eventually(t, func() bool {
		started, _ := underTest.backtestRunApplication.RunClaimedBacktests(t.Context())
		return started == 1
	}, 5*time.Second, 10*time.Millisecond, "the waiting run starts once a place is free")
}

func TestRunClaimedBacktestsPassesOnAQueueThatCannotBeRead(t *testing.T) {
	underTest := newBacktestRunWorkersUnderTest(t, 2)
	underTest.runRepository.EXPECT().ClaimOldestQueued(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entities.BacktestRun{}, false, errors.New("storage unavailable"))

	started, err := underTest.backtestRunApplication.RunClaimedBacktests(t.Context())

	assert.Error(t, err)
	assert.Equal(t, 0, started)
}

func TestDeleteExpiredRunsUsesTheRetention(t *testing.T) {
	underTest := newBacktestRunWorkersUnderTest(t, 2)
	underTest.runRepository.EXPECT().DeleteFinishedBefore(gomock.Any(), backtestNow.Add(-30*24*time.Hour)).Return(3, nil)

	deleted, err := underTest.backtestRunApplication.DeleteExpiredRuns(t.Context())

	require.NoError(t, err)
	assert.Equal(t, 3, deleted)
}
