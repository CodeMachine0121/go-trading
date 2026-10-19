package application

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type SymbolScreeningApplication struct {
	profileService      *service.SymbolScreeningProfileService
	runService          *service.SymbolScreeningRunService
	marketDataService   *service.SymbolScreeningMarketDataService
	runRetention        time.Duration
	marketDataRetention time.Duration
}

func NewSymbolScreeningApplication(
	profileService *service.SymbolScreeningProfileService,
	runService *service.SymbolScreeningRunService,
	marketDataService *service.SymbolScreeningMarketDataService,
	runRetention time.Duration,
	marketDataRetention time.Duration,
) *SymbolScreeningApplication {
	return &SymbolScreeningApplication{
		profileService:      profileService,
		runService:          runService,
		marketDataService:   marketDataService,
		runRetention:        runRetention,
		marketDataRetention: marketDataRetention,
	}
}

func (screeningApplication *SymbolScreeningApplication) ListProfiles(
	executionContext context.Context, viewerID uint,
) ([]dto.SymbolScreeningProfileDto, error) {
	return screeningApplication.profileService.List(executionContext, viewerID)
}

func (screeningApplication *SymbolScreeningApplication) GetProfile(
	executionContext context.Context, viewerID uint, id uint,
) (dto.SymbolScreeningProfileDto, error) {
	return screeningApplication.profileService.Get(executionContext, viewerID, id)
}

func (screeningApplication *SymbolScreeningApplication) CreateProfile(
	executionContext context.Context, viewerID uint, writeDto dto.SymbolScreeningProfileWriteDto,
) (dto.SymbolScreeningProfileDto, error) {
	return screeningApplication.profileService.Create(executionContext, viewerID, writeDto)
}

func (screeningApplication *SymbolScreeningApplication) UpdateProfile(
	executionContext context.Context, viewerID uint, id uint, writeDto dto.SymbolScreeningProfileWriteDto,
) (dto.SymbolScreeningProfileDto, error) {
	return screeningApplication.profileService.Update(executionContext, viewerID, id, writeDto)
}

func (screeningApplication *SymbolScreeningApplication) DeleteProfile(
	executionContext context.Context, viewerID uint, id uint,
) error {
	return screeningApplication.profileService.Delete(executionContext, viewerID, id)
}

// SubmitRun freezes the profile and its script first, so the queued run never depends on later edits.
func (screeningApplication *SymbolScreeningApplication) SubmitRun(
	executionContext context.Context, viewerID uint, submission dto.SymbolScreeningRunSubmissionDto,
) (dto.SymbolScreeningRunDto, error) {
	snapshot, snapshotError := screeningApplication.profileService.SnapshotFor(
		executionContext, viewerID, submission.ProfileID)
	if snapshotError != nil {
		return dto.SymbolScreeningRunDto{}, snapshotError
	}

	return screeningApplication.runService.Submit(
		executionContext, viewerID, submission.ProfileID, snapshot, submission.SymbolList)
}

func (screeningApplication *SymbolScreeningApplication) ListRuns(
	executionContext context.Context, viewerID uint, query dto.SymbolScreeningRunListQueryDto,
) (dto.SymbolScreeningRunPageDto, error) {
	return screeningApplication.runService.List(executionContext, viewerID, query)
}

func (screeningApplication *SymbolScreeningApplication) GetRun(
	executionContext context.Context, viewerID uint, id uint, wait time.Duration,
) (dto.SymbolScreeningRunDetailDto, error) {
	return screeningApplication.runService.Get(executionContext, viewerID, id, wait)
}

func (screeningApplication *SymbolScreeningApplication) CancelRun(
	executionContext context.Context, viewerID uint, id uint,
) (dto.SymbolScreeningRunDto, error) {
	return screeningApplication.runService.Cancel(executionContext, viewerID, id)
}

func (screeningApplication *SymbolScreeningApplication) RunClaimedScreening(executionContext context.Context) (bool, error) {
	return screeningApplication.runService.ClaimAndRun(executionContext)
}

func (screeningApplication *SymbolScreeningApplication) StopWaiting() {
	screeningApplication.runService.StopWaiting()
}

func (screeningApplication *SymbolScreeningApplication) StopRunner() {
	screeningApplication.runService.StopRunner()
}

func (screeningApplication *SymbolScreeningApplication) DeleteExpired(executionContext context.Context) (int, int, error) {
	deletedRuns, deleteRunsError := screeningApplication.runService.DeleteExpiredRuns(
		executionContext, screeningApplication.runRetention)
	if deleteRunsError != nil {
		return 0, 0, deleteRunsError
	}
	deletedMarketData, deleteMarketDataError := screeningApplication.marketDataService.DeleteExpired(
		executionContext, screeningApplication.marketDataRetention)

	return deletedRuns, deletedMarketData, deleteMarketDataError
}
