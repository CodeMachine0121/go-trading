package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// KCandleContractIngestionApplication orchestrates automatic contract K candle
// ingestion. Each method is one call into the domain; the ordering between backfill
// and the periodic rounds belongs to whoever drives them, not here.
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

// CatchUpSymbol closes one contract's gap on demand, for somebody who wants its
// history now rather than at the next start-up.
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

// StartSymbolHistorySync accepts a request to fill in the minutes missing from a
// named stretch of one contract's history, and answers with the run to watch.
//
// The ceiling travels through rather than being held anywhere in the middle: it is an
// operator's decision, settled once at the composition root.
func (kCandleContractIngestionApplication *KCandleContractIngestionApplication) StartSymbolHistorySync(
	executionContext context.Context, syncDto dto.KCandleHistorySyncDto, lookbackCeilingDays int,
) (dto.KCandleHistorySyncRunDto, error) {
	return kCandleContractIngestionApplication.contractKCandleIngestionService.StartHistorySyncFor(
		executionContext, syncDto, lookbackCeilingDays)
}

// GetSymbolHistorySync answers with where one contract history sync has got to.
func (kCandleContractIngestionApplication *KCandleContractIngestionApplication) GetSymbolHistorySync(
	executionContext context.Context, id uint,
) (dto.KCandleHistorySyncRunDto, error) {
	return kCandleContractIngestionApplication.contractKCandleIngestionService.GetHistorySyncRun(
		executionContext, id)
}

// FailInterruptedHistorySyncs clears out the contract runs the last shutdown cut off,
// and says how many there were so that whoever starts the system can say it out loud.
func (kCandleContractIngestionApplication *KCandleContractIngestionApplication) FailInterruptedHistorySyncs(
	executionContext context.Context,
) (int, error) {
	return kCandleContractIngestionApplication.contractKCandleIngestionService.
		FailInterruptedHistorySyncs(executionContext)
}
