package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type KCandleSummaryApplication struct {
	kCandleSummaryService *service.KCandleSummaryService
}

func NewKCandleSummaryApplication(kCandleSummaryService *service.KCandleSummaryService) *KCandleSummaryApplication {
	return &KCandleSummaryApplication{kCandleSummaryService: kCandleSummaryService}
}

func (kCandleSummaryApplication *KCandleSummaryApplication) RunRound(executionContext context.Context) error {
	return kCandleSummaryApplication.kCandleSummaryService.SummarizeRecentDays(executionContext)
}
