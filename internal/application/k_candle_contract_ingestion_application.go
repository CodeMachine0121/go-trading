package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type KCandleContractIngestionApplication struct {
	contractKCandleIngestionService *service.ContractKCandleIngestionService
}

func NewKCandleContractIngestionApplication(
	contractKCandleIngestionService *service.ContractKCandleIngestionService,
) *KCandleContractIngestionApplication {
	return &KCandleContractIngestionApplication{
		contractKCandleIngestionService: contractKCandleIngestionService,
	}
}

func (kCandleContractIngestionApplication *KCandleContractIngestionApplication) RunBackfill(
	executionContext context.Context,
) (dto.KCandleIngestionReportDto, error) {
	return kCandleContractIngestionApplication.contractKCandleIngestionService.RunBackfill(
		executionContext)
}

func (kCandleContractIngestionApplication *KCandleContractIngestionApplication) CatchUpSymbol(
	executionContext context.Context, symbol string,
) (dto.KCandleIngestionReportDto, error) {
	return kCandleContractIngestionApplication.contractKCandleIngestionService.RunBackfillFor(
		executionContext, symbol)
}

func (kCandleContractIngestionApplication *KCandleContractIngestionApplication) RunScheduledRound(
	executionContext context.Context,
) (dto.KCandleIngestionReportDto, error) {
	return kCandleContractIngestionApplication.contractKCandleIngestionService.RunScheduledRound(
		executionContext)
}

// StartSymbolHistorySync starts filling missing minutes in a stretch of one contract's history and returns the run to watch; the ceiling is passed in from the composition root.
func (kCandleContractIngestionApplication *KCandleContractIngestionApplication) StartSymbolHistorySync(
	executionContext context.Context, syncDto dto.KCandleHistorySyncDto, lookbackCeilingDays int,
) (dto.KCandleContractHistorySyncRunDto, error) {
	return kCandleContractIngestionApplication.contractKCandleIngestionService.StartHistorySyncFor(
		executionContext, syncDto, lookbackCeilingDays)
}

func (kCandleContractIngestionApplication *KCandleContractIngestionApplication) GetSymbolHistorySync(
	executionContext context.Context, id uint,
) (dto.KCandleContractHistorySyncRunDto, error) {
	return kCandleContractIngestionApplication.contractKCandleIngestionService.GetHistorySyncRun(
		executionContext, id)
}

// FailInterruptedHistorySyncs fails runs the last shutdown cut off and returns how many.
func (kCandleContractIngestionApplication *KCandleContractIngestionApplication) FailInterruptedHistorySyncs(
	executionContext context.Context,
) (int, error) {
	return kCandleContractIngestionApplication.contractKCandleIngestionService.
		FailInterruptedHistorySyncs(executionContext)
}
