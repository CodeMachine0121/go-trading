package application

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// KCandleContractApplication orchestrates the perpetual contract K candle use cases.
// Each method is one call into the domain; no rule, ordering or limit decision lives
// here.
type KCandleContractApplication struct {
	kCandleContractService *service.KCandleContractService
}

func NewKCandleContractApplication(
	kCandleContractService *service.KCandleContractService,
) *KCandleContractApplication {
	return &KCandleContractApplication{kCandleContractService: kCandleContractService}
}

func (kCandleContractApplication *KCandleContractApplication) SaveKCandleContract(
	executionContext context.Context, writeDto dto.KCandleContractWriteDto,
) (dto.KCandleContractDto, error) {
	return kCandleContractApplication.kCandleContractService.SaveKCandleContract(
		executionContext, writeDto)
}

// GetKCandleContractSeries reads one stretch of contract K candles merged by interval.
func (kCandleContractApplication *KCandleContractApplication) GetKCandleContractSeries(
	executionContext context.Context, seriesQueryDto dto.KCandleSeriesQueryDto,
) (dto.KCandleContractSeriesDto, error) {
	return kCandleContractApplication.kCandleContractService.GetKCandleContractSeries(
		executionContext, seriesQueryDto)
}

func (kCandleContractApplication *KCandleContractApplication) GetKCandleContractsInRange(
	executionContext context.Context, queryDto dto.KCandleQueryDto,
) ([]dto.KCandleContractDto, error) {
	return kCandleContractApplication.kCandleContractService.GetKCandleContractsInRange(
		executionContext, queryDto)
}

func (kCandleContractApplication *KCandleContractApplication) GetKCandleContract(
	executionContext context.Context, symbol string, openTime time.Time,
) (dto.KCandleContractDto, error) {
	return kCandleContractApplication.kCandleContractService.GetKCandleContract(
		executionContext, symbol, openTime)
}

func (kCandleContractApplication *KCandleContractApplication) UpdateKCandleContract(
	executionContext context.Context, writeDto dto.KCandleContractWriteDto,
) (dto.KCandleContractDto, error) {
	return kCandleContractApplication.kCandleContractService.UpdateKCandleContract(
		executionContext, writeDto)
}

func (kCandleContractApplication *KCandleContractApplication) DeleteKCandleContract(
	executionContext context.Context, symbol string, openTime time.Time,
) error {
	return kCandleContractApplication.kCandleContractService.DeleteKCandleContract(
		executionContext, symbol, openTime)
}
