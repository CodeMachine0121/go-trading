package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// KCandleIngestionApplication orchestrates automatic K candle ingestion. Each method
// is one call into the domain; the ordering between backfill and the periodic rounds
// belongs to whoever drives them, not here.
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

// CatchUpSymbol closes one trading symbol's gap on demand, for somebody who wants
// its history now rather than at the next start-up.
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

// SyncSymbolHistory fetches a named stretch of one trading symbol's history and
// stores it over whatever was there.
//
// The ceiling travels through rather than being held anywhere in the middle: it is an
// operator's decision, settled once at the composition root, and a layer that
// remembered a copy would be a second place for it to drift.
func (kCandleIngestionApplication *KCandleIngestionApplication) SyncSymbolHistory(
	executionContext context.Context, syncDto dto.KCandleHistorySyncDto, lookbackCeilingDays int,
) (dto.KCandleIngestionReportDto, error) {
	return kCandleIngestionApplication.kCandleIngestionService.SyncHistoryFor(
		executionContext, syncDto, lookbackCeilingDays)
}
