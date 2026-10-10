package application

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// BacktestRunApplication is how people watch and cancel their runs, and how the worker and retention jobs drive them.
type BacktestRunApplication struct {
	backtestRunService *service.BacktestRunService
	workerCount        int
	retention          time.Duration
}

func NewBacktestRunApplication(
	backtestRunService *service.BacktestRunService, workerCount int, retention time.Duration,
) *BacktestRunApplication {
	return &BacktestRunApplication{
		backtestRunService: backtestRunService,
		workerCount:        max(workerCount, 1),
		retention:          retention,
	}
}

func (backtestRunApplication *BacktestRunApplication) ListRuns(
	executionContext context.Context, viewerID uint, query dto.BacktestRunListQueryDto,
) (dto.BacktestRunPageDto, error) {
	return backtestRunApplication.backtestRunService.List(executionContext, viewerID, query)
}

func (backtestRunApplication *BacktestRunApplication) GetRun(
	executionContext context.Context, viewerID uint, id uint, wait time.Duration,
) (dto.BacktestRunDetailDto, error) {
	return backtestRunApplication.backtestRunService.Get(executionContext, viewerID, id, wait)
}

func (backtestRunApplication *BacktestRunApplication) CancelRun(
	executionContext context.Context, viewerID uint, id uint,
) (dto.BacktestRunDto, error) {
	return backtestRunApplication.backtestRunService.Cancel(executionContext, viewerID, id)
}

// RunClaimedBacktests fills this replica's free worker places from the queue and says how many runs it started.
func (backtestRunApplication *BacktestRunApplication) RunClaimedBacktests(
	executionContext context.Context,
) (int, error) {
	startedCount := 0
	for backtestRunApplication.backtestRunService.RunningCount() < backtestRunApplication.workerCount {
		started, claimError := backtestRunApplication.backtestRunService.ClaimAndRun(executionContext)
		if claimError != nil || !started {
			return startedCount, claimError
		}
		startedCount++
	}

	return startedCount, nil
}

// StopRunners cuts off this replica's runs on a graceful stop, marking each interrupted.
func (backtestRunApplication *BacktestRunApplication) StopRunners() {
	backtestRunApplication.backtestRunService.StopRunners()
}

func (backtestRunApplication *BacktestRunApplication) DeleteExpiredRuns(executionContext context.Context) (int, error) {
	return backtestRunApplication.backtestRunService.DeleteExpiredRuns(executionContext, backtestRunApplication.retention)
}
