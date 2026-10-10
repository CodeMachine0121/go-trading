package application_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"go.uber.org/mock/gomock"
)

const backtestRunOwnerActiveLimit = 3

// backtestRunStore is the queue as mocks: it remembers what was saved so a test can say "nothing was queued" as easily as "this was".
type backtestRunStore struct {
	runRepository    *mocks.MockIBacktestRunRepository
	resultRepository *mocks.MockIBacktestRunResultRepository
	mutex            sync.Mutex
	saved            []entities.BacktestRun
	activeCount      int
}

func newBacktestRunStore(controller *gomock.Controller) *backtestRunStore {
	store := &backtestRunStore{
		runRepository:    mocks.NewMockIBacktestRunRepository(controller),
		resultRepository: mocks.NewMockIBacktestRunResultRepository(controller),
	}
	store.runRepository.EXPECT().CountActiveByOwner(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, uint) (int, error) { return store.activeCount, nil }).AnyTimes()
	store.runRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, backtestRun entities.BacktestRun) (entities.BacktestRun, error) {
			store.mutex.Lock()
			defer store.mutex.Unlock()
			backtestRun.ID = uint(len(store.saved) + 1)
			store.saved = append(store.saved, backtestRun)

			return backtestRun, nil
		}).AnyTimes()

	return store
}

func (store *backtestRunStore) savedRuns() []entities.BacktestRun {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	return append([]entities.BacktestRun{}, store.saved...)
}

// newQueueingBacktestRunService builds the queue over the store with a transaction that simply runs its work.
func newQueueingBacktestRunService(
	t *testing.T,
	controller *gomock.Controller,
	store *backtestRunStore,
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
		store.runRepository, store.resultRepository, transactionRepository, clockProxy,
		backtestService, contractBacktestService,
		domains.NewBacktestRunAdmissionDomain(backtestRunOwnerActiveLimit),
		"replica-test", 30*time.Minute, 100)
}
