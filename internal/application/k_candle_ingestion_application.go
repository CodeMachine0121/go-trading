package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type KCandleIngestionApplication struct {
	kCandleIngestionService *service.KCandleIngestionService
}

func NewKCandleIngestionApplication(
	kCandleIngestionService *service.KCandleIngestionService,
) *KCandleIngestionApplication {
	return &KCandleIngestionApplication{kCandleIngestionService: kCandleIngestionService}
}

func (kCandleIngestionApplication *KCandleIngestionApplication) RunBackfill(
	executionContext context.Context,
) (dto.KCandleIngestionReportDto, error) {
	return kCandleIngestionApplication.kCandleIngestionService.RunBackfill(executionContext)
}

func (kCandleIngestionApplication *KCandleIngestionApplication) CatchUpSymbol(
	executionContext context.Context, symbol string,
) (dto.KCandleIngestionReportDto, error) {
	return kCandleIngestionApplication.kCandleIngestionService.RunBackfillFor(
		executionContext, symbol)
}

func (kCandleIngestionApplication *KCandleIngestionApplication) RunScheduledRound(
	executionContext context.Context,
) (dto.KCandleIngestionReportDto, error) {
	return kCandleIngestionApplication.kCandleIngestionService.RunScheduledRound(executionContext)
}

// StartSymbolHistorySync starts filling missing minutes in a stretch of one symbol's history, leaving held candles untouched, and returns the run to watch.
func (kCandleIngestionApplication *KCandleIngestionApplication) StartSymbolHistorySync(
	executionContext context.Context, syncDto dto.KCandleHistorySyncDto, lookbackCeilingDays int,
) (dto.KCandleHistorySyncRunDto, error) {
	return kCandleIngestionApplication.kCandleIngestionService.StartHistorySyncFor(
		executionContext, syncDto, lookbackCeilingDays)
}

func (kCandleIngestionApplication *KCandleIngestionApplication) GetSymbolHistorySync(
	executionContext context.Context, id uint,
) (dto.KCandleHistorySyncRunDto, error) {
	return kCandleIngestionApplication.kCandleIngestionService.GetHistorySyncRun(
		executionContext, id)
}

// FailInterruptedHistorySyncs fails runs the last shutdown cut off and returns how many.
func (kCandleIngestionApplication *KCandleIngestionApplication) FailInterruptedHistorySyncs(
	executionContext context.Context,
) (int, error) {
	return kCandleIngestionApplication.kCandleIngestionService.FailInterruptedHistorySyncs(
		executionContext)
}
