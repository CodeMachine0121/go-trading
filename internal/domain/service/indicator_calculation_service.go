package service

import (
	"context"

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
// trading symbol, leaving out the newest candle, and reports one value per indicator
// name in the kind the request declared. An empty set of names is a valid result.
func (indicatorCalculationService *IndicatorCalculationService) CalculateIndicator(
	executionContext context.Context, requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
		requestDto, indicatorCalculationService.maxCandleCount)
	if validationError != nil {
		return dto.IndicatorCalculationResultDto{}, validationError
	}

	newestFirstKCandles, findError := indicatorCalculationService.kCandleRepository.FindLatest(
		executionContext, calculationDomain.Symbol(), calculationDomain.CandleFetchCount())
	if findError != nil {
		return dto.IndicatorCalculationResultDto{}, findError
	}

	inputKCandleVos, selectionError := calculationDomain.SelectInputCandles(newestFirstKCandles)
	if selectionError != nil {
		return dto.IndicatorCalculationResultDto{}, selectionError
	}

	indicatorValues, executionError := indicatorCalculationService.indicatorScriptProxy.Execute(
		executionContext, requestDto.Script, calculationDomain.ResultType(), inputKCandleVos)
	if executionError != nil {
		return dto.IndicatorCalculationResultDto{}, executionError
	}

	indicatorValueDtos := make(map[string]dto.IndicatorValueDto, len(indicatorValues))
	for indicatorName, indicatorValue := range indicatorValues {
		indicatorValueDtos[indicatorName] = indicatorValue.ToDto()
	}

	return dto.IndicatorCalculationResultDto{
		Symbol:          calculationDomain.Symbol(),
		UsedCandleCount: len(inputKCandleVos),
		ResultType:      string(calculationDomain.ResultType().Value()),
		Values:          indicatorValueDtos,
	}, nil
}
