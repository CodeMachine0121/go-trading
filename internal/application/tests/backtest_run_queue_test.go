package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"go.uber.org/mock/gomock"
)

const backtestRunOwnerActiveLimit = 3

// backtestRunQueue lets each case state what it expects of the queue: one run saved, a refusal at the limit, or nothing at all.
type backtestRunQueue struct {
	runRepository    *mocks.MockIBacktestRunRepository
	resultRepository *mocks.MockIBacktestRunResultRepository
}

func newBacktestRunQueue(controller *gomock.Controller) backtestRunQueue {
	return backtestRunQueue{
		runRepository:    mocks.NewMockIBacktestRunRepository(controller),
		resultRepository: mocks.NewMockIBacktestRunResultRepository(controller),
	}
}

// acceptsOneRun expects the owner, holding activeCount runs, to have exactly one run saved, and hands back the run as saved.
func (queue backtestRunQueue) acceptsOneRun(activeCount int) *entities.BacktestRun {
	saved := &entities.BacktestRun{}
	queue.runRepository.EXPECT().CountActiveByOwner(gomock.Any(), gomock.Any()).Return(activeCount, nil)
	queue.runRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, backtestRun entities.BacktestRun) (entities.BacktestRun, error) {
			backtestRun.ID = 1
			*saved = backtestRun
			return backtestRun, nil
		})

	return saved
}

func (queue backtestRunQueue) refusesAtTheLimit() {
	queue.runRepository.EXPECT().CountActiveByOwner(gomock.Any(), gomock.Any()).Return(backtestRunOwnerActiveLimit, nil)
	queue.runRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)
}

func (queue backtestRunQueue) queuesNothing() {
	queue.runRepository.EXPECT().CountActiveByOwner(gomock.Any(), gomock.Any()).Times(0)
	queue.runRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)
}

// newQueueingBacktestRunService builds the queue over the mocks with a transaction that simply runs its work.
func newQueueingBacktestRunService(
	t *testing.T,
	controller *gomock.Controller,
	queue backtestRunQueue,
	backtestService *service.BacktestService,
	contractBacktestService *service.ContractBacktestService,
) *service.BacktestRunService {
	t.Helper()
	transactionRepository := mocks.NewMockITransactionRepository(controller)
	transactionRepository.EXPECT().Atomically(gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, work func(context.Context) error) error {
			return work(executionContext)
		}).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(backtestNow).AnyTimes()

	return service.NewBacktestRunService(
		queue.runRepository, queue.resultRepository, transactionRepository, clockProxy,
		backtestService, contractBacktestService,
		domains.NewBacktestRunAdmissionDomain(backtestRunOwnerActiveLimit),
		"replica-test", 30*time.Minute, 100, time.Minute)
}
