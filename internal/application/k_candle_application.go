package application

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// KCandleApplication orchestrates the K candle use cases. Each method is one call
// into the domain; no rule, ordering or limit decision lives here.
type KCandleApplication struct {
	kCandleService *service.KCandleService
}

func NewKCandleApplication(kCandleService *service.KCandleService) *KCandleApplication {
	return &KCandleApplication{kCandleService: kCandleService}
}

func (kCandleApplication *KCandleApplication) SaveKCandle(
	executionContext context.Context, writeDto dto.KCandleWriteDto,
) (dto.KCandleDto, error) {
	return kCandleApplication.kCandleService.SaveKCandle(executionContext, writeDto)
}

func (kCandleApplication *KCandleApplication) GetKCandlesInRange(
	executionContext context.Context, queryDto dto.KCandleQueryDto,
) ([]dto.KCandleDto, error) {
	return kCandleApplication.kCandleService.GetKCandlesInRange(executionContext, queryDto)
}

func (kCandleApplication *KCandleApplication) GetKCandleSeries(
	executionContext context.Context, seriesQueryDto dto.KCandleSeriesQueryDto,
) (dto.KCandleSeriesDto, error) {
	return kCandleApplication.kCandleService.GetKCandleSeries(executionContext, seriesQueryDto)
}

func (kCandleApplication *KCandleApplication) GetKCandle(
	executionContext context.Context, symbol string, openTime time.Time,
) (dto.KCandleDto, error) {
	return kCandleApplication.kCandleService.GetKCandle(executionContext, symbol, openTime)
}

func (kCandleApplication *KCandleApplication) UpdateKCandle(
	executionContext context.Context, writeDto dto.KCandleWriteDto,
) (dto.KCandleDto, error) {
	return kCandleApplication.kCandleService.UpdateKCandle(executionContext, writeDto)
}

func (kCandleApplication *KCandleApplication) DeleteKCandle(
	executionContext context.Context, symbol string, openTime time.Time,
) error {
	return kCandleApplication.kCandleService.DeleteKCandle(executionContext, symbol, openTime)
}
