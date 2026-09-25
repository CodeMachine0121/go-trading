package service

import (
	"context"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

type IndicatorCalculationService struct {
	kCandleRepository       domaininterface.IKCandleRepository
	tradingSymbolRepository domaininterface.ITradingSymbolRepository
	indicatorScriptProxy    domaininterface.IIndicatorScriptProxy
	clockProxy              domaininterface.IClockProxy
	marketCatalogDomain     domains.MarketCatalogDomain
	maxCandleCount          int
}

func NewIndicatorCalculationService(
	kCandleRepository domaininterface.IKCandleRepository,
	tradingSymbolRepository domaininterface.ITradingSymbolRepository,
	indicatorScriptProxy domaininterface.IIndicatorScriptProxy,
	clockProxy domaininterface.IClockProxy,
	marketCatalogDomain domains.MarketCatalogDomain,
	maxCandleCount int,
) *IndicatorCalculationService {
	return &IndicatorCalculationService{
		kCandleRepository:       kCandleRepository,
		tradingSymbolRepository: tradingSymbolRepository,
		indicatorScriptProxy:    indicatorScriptProxy,
		clockProxy:              clockProxy,
		marketCatalogDomain:     marketCatalogDomain,
		maxCandleCount:          maxCandleCount,
	}
}

// CalculateIndicator runs the script over the requested number of finished aggregated candles, answering over what is stored and reporting expected versus used bucket counts.
//
// Results are repeatable only for settled stretches: at the live edge a bucket may count as finished before its last candle is ingested, so callers needing a stable answer should pass a past end time.
func (indicatorCalculationService *IndicatorCalculationService) CalculateIndicator(
	executionContext context.Context, requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	// An unregistered symbol reads as the round-the-clock market, matching its behaviour before markets existed.
	registeredSymbol, _, findSymbolError := indicatorCalculationService.tradingSymbolRepository.
		FindBySymbol(executionContext, requestDto.Symbol)
	if findSymbolError != nil {
		return dto.IndicatorCalculationResultDto{}, findSymbolError
	}

	marketDomain := indicatorCalculationService.marketCatalogDomain.MarketOf(registeredSymbol.Market)

	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
		requestDto,
		marketDomain,
		indicatorCalculationService.maxCandleCount,
		indicatorCalculationService.clockProxy.Now(),
	)
	if validationError != nil {
		return dto.IndicatorCalculationResultDto{}, validationError
	}

	newestFirstKCandles, findError := indicatorCalculationService.kCandleRepository.FindLatestBefore(
		executionContext,
		calculationDomain.Symbol(),
		calculationDomain.ReadCutoff(),
		calculationDomain.SourceCandleLimit(),
	)
	if findError != nil {
		return dto.IndicatorCalculationResultDto{}, findError
	}

	inputKCandleVos, selectionError := calculationDomain.SelectInputCandles(newestFirstKCandles)
	if selectionError != nil {
		return dto.IndicatorCalculationResultDto{}, selectionError
	}

	indicatorValues, executionError := indicatorCalculationService.indicatorScriptProxy.Execute(
		executionContext, requestDto.Script, calculationDomain.ResultType(), inputKCandleVos,
		calculationDomain.Parameters())
	if executionError != nil {
		return dto.IndicatorCalculationResultDto{}, executionError
	}

	// Candle open times in script order, so callers can align list values.
	openTimes := make([]time.Time, 0, len(inputKCandleVos))
	for _, inputKCandleVo := range inputKCandleVos {
		openTimes = append(openTimes, time.Unix(inputKCandleVo.OpenTimeUnixSeconds, 0).UTC())
	}

	// Both counts come from the calculation itself.
	return calculationDomain.ToResultDto(openTimes, indicatorValues), nil
}
