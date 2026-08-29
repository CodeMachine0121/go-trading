package application

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// IndicatorCalculationApplication orchestrates the indicator calculation use case.
type IndicatorCalculationApplication struct {
	indicatorCalculationService *service.IndicatorCalculationService
}

func NewIndicatorCalculationApplication(
	indicatorCalculationService *service.IndicatorCalculationService,
) *IndicatorCalculationApplication {
	return &IndicatorCalculationApplication{indicatorCalculationService: indicatorCalculationService}
}

func (indicatorCalculationApplication *IndicatorCalculationApplication) CalculateIndicator(
	requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	return indicatorCalculationApplication.indicatorCalculationService.CalculateIndicator(requestDto)
}
