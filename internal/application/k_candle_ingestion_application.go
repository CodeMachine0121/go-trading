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
	executionContext context.Context, symbols []string,
) (dto.KCandleIngestionReportDto, error) {
	return kCandleIngestionApplication.kCandleIngestionService.RunBackfill(executionContext, symbols)
}

func (kCandleIngestionApplication *KCandleIngestionApplication) RunScheduledRound(
	executionContext context.Context, symbols []string,
) (dto.KCandleIngestionReportDto, error) {
	return kCandleIngestionApplication.kCandleIngestionService.RunScheduledRound(
		executionContext, symbols)
}
