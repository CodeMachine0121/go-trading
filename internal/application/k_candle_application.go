package application

import (
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

func (kCandleApplication *KCandleApplication) SaveKCandle(writeDto dto.KCandleWriteDto) (dto.KCandleDto, error) {
	return kCandleApplication.kCandleService.SaveKCandle(writeDto)
}

func (kCandleApplication *KCandleApplication) GetKCandlesInRange(queryDto dto.KCandleQueryDto) ([]dto.KCandleDto, error) {
	return kCandleApplication.kCandleService.GetKCandlesInRange(queryDto)
}

func (kCandleApplication *KCandleApplication) GetKCandle(symbol string, openTime time.Time) (dto.KCandleDto, error) {
	return kCandleApplication.kCandleService.GetKCandle(symbol, openTime)
}

func (kCandleApplication *KCandleApplication) UpdateKCandle(writeDto dto.KCandleWriteDto) (dto.KCandleDto, error) {
	return kCandleApplication.kCandleService.UpdateKCandle(writeDto)
}

func (kCandleApplication *KCandleApplication) DeleteKCandle(symbol string, openTime time.Time) error {
	return kCandleApplication.kCandleService.DeleteKCandle(symbol, openTime)
}
