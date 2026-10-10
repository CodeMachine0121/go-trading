package controller_test

import (
	"context"
	"testing"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"go.uber.org/mock/gomock"
)

const (
	routerBacktestRunOwnerActiveLimit = 3
	queuedBacktestRunID               = uint(31)
)

// routerBacktestRuns is the real run service over mocked storage; a submit that should leave nothing behind simply never expects a save.
type routerBacktestRuns struct {
	backtestRunService *service.BacktestRunService
	runRepository      *mocks.MockIBacktestRunRepository
	resultRepository   *mocks.MockIBacktestRunResultRepository
}

func newRouterBacktestRuns(
	t *testing.T,
	mockController *gomock.Controller,
	clockProxy domaininterface.IClockProxy,
	backtestService *service.BacktestService,
	contractBacktestService *service.ContractBacktestService,
) routerBacktestRuns {
	t.Helper()
	runRepository := mocks.NewMockIBacktestRunRepository(mockController)
	resultRepository := mocks.NewMockIBacktestRunResultRepository(mockController)
	transactionRepository := mocks.NewMockITransactionRepository(mockController)
	transactionRepository.EXPECT().Atomically(gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, work func(context.Context) error) error {
			return work(executionContext)
		}).AnyTimes()

	return routerBacktestRuns{
		backtestRunService: service.NewBacktestRunService(
			runRepository, resultRepository, transactionRepository, clockProxy,
			backtestService, contractBacktestService,
			domains.NewBacktestRunAdmissionDomain(routerBacktestRunOwnerActiveLimit),
			"replica-a", 30*time.Minute, 100),
		runRepository:    runRepository,
		resultRepository: resultRepository,
	}
}

// ownerHasActive answers the per-owner count the submit checks against the limit.
func (runs routerBacktestRuns) ownerHasActive(activeCount int) {
	runs.runRepository.EXPECT().CountActiveByOwner(gomock.Any(), signedInViewerID).
		Return(activeCount, nil).AnyTimes()
}

// expectQueued accepts exactly one save and hands back what was saved.
func (runs routerBacktestRuns) expectQueued() *entities.BacktestRun {
	runs.ownerHasActive(0)
	saved := &entities.BacktestRun{}
	runs.runRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, backtestRun entities.BacktestRun) (entities.BacktestRun, error) {
			backtestRun.ID = queuedBacktestRunID
			*saved = backtestRun

			return backtestRun, nil
		})

	return saved
}
