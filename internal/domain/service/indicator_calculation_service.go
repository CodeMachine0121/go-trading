package service

import (
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// IndicatorCalculationService is the application layer's only entry point for
// running a user-written indicator script.
type IndicatorCalculationService struct {
	kCandleRepository    domaininterface.IKCandleRepository
	indicatorScriptProxy domaininterface.IIndicatorScriptProxy
	maxCandleCount       int
}

func NewIndicatorCalculationService(
	kCandleRepository domaininterface.IKCandleRepository,
	indicatorScriptProxy domaininterface.IIndicatorScriptProxy,
	maxCandleCount int,
) *IndicatorCalculationService {
	return &IndicatorCalculationService{
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
		maxCandleCount:       maxCandleCount,
	}
}

// CalculateIndicator runs the script over the requested number of K candles for the
// trading symbol, leaving out the newest candle, and reports one number per
// indicator name. An empty set of names is a valid result.
func (indicatorCalculationService *IndicatorCalculationService) CalculateIndicator(
	requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
		requestDto, indicatorCalculationService.maxCandleCount)
	if validationError != nil {
		return dto.IndicatorCalculationResultDto{}, validationError
	}

	newestFirstKCandles, findError := indicatorCalculationService.kCandleRepository.FindLatest(
		calculationDomain.Symbol(), calculationDomain.CandleFetchCount())
	if findError != nil {
		return dto.IndicatorCalculationResultDto{}, findError
	}

	inputKCandleVos, selectionError := calculationDomain.SelectInputCandles(newestFirstKCandles)
	if selectionError != nil {
		return dto.IndicatorCalculationResultDto{}, selectionError
	}

	indicatorValues, executionError := indicatorCalculationService.indicatorScriptProxy.Execute(
		requestDto.Script, inputKCandleVos)
	if executionError != nil {
		return dto.IndicatorCalculationResultDto{}, executionError
	}

	return dto.IndicatorCalculationResultDto{
		Symbol:          calculationDomain.Symbol(),
		UsedCandleCount: len(inputKCandleVos),
		Values:          indicatorValues,
	}, nil
}
