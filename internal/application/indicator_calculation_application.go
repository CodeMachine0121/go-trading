package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// IndicatorCalculationApplication joins script resolution and calculation, so another person's published script can run without its source leaving the server.
type IndicatorCalculationApplication struct {
	strategyScriptService               *service.StrategyScriptService
	indicatorCalculationService         *service.IndicatorCalculationService
	contractIndicatorCalculationService *service.ContractIndicatorCalculationService
}

func NewIndicatorCalculationApplication(
	strategyScriptService *service.StrategyScriptService,
	indicatorCalculationService *service.IndicatorCalculationService,
	contractIndicatorCalculationService *service.ContractIndicatorCalculationService,
) *IndicatorCalculationApplication {
	return &IndicatorCalculationApplication{
		strategyScriptService:               strategyScriptService,
		indicatorCalculationService:         indicatorCalculationService,
		contractIndicatorCalculationService: contractIndicatorCalculationService,
	}
}

// CalculateIndicator runs either the named strategy script (through the visibility gates) or the caller's own algorithm over spot K candles.
func (indicatorCalculationApplication *IndicatorCalculationApplication) CalculateIndicator(
	executionContext context.Context,
	viewerID uint,
	runSubjectDomain domains.RunSubjectDomain,
	requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	runnableRequestDto, resolveError := indicatorCalculationApplication.resolveRunnable(
		executionContext, viewerID, runSubjectDomain, vo.MarketDataKindKCandle, requestDto)
	if resolveError != nil {
		return dto.IndicatorCalculationResultDto{}, resolveError
	}

	return indicatorCalculationApplication.indicatorCalculationService.CalculateIndicator(
		executionContext, runnableRequestDto)
}

// CalculateContractIndicator is CalculateIndicator over perpetual contract bars.
func (indicatorCalculationApplication *IndicatorCalculationApplication) CalculateContractIndicator(
	executionContext context.Context,
	viewerID uint,
	runSubjectDomain domains.RunSubjectDomain,
	requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	runnableRequestDto, resolveError := indicatorCalculationApplication.resolveRunnable(
		executionContext, viewerID, runSubjectDomain, vo.MarketDataKindContractKCandle, requestDto)
	if resolveError != nil {
		return dto.IndicatorCalculationResultDto{}, resolveError
	}

	return indicatorCalculationApplication.contractIndicatorCalculationService.CalculateContractIndicator(
		executionContext, runnableRequestDto)
}

// resolveRunnable refuses a named script built for the other market kind so it isn't misreported as broken; the caller's own algorithm is assumed to target this market.
func (indicatorCalculationApplication *IndicatorCalculationApplication) resolveRunnable(
	executionContext context.Context,
	viewerID uint,
	runSubjectDomain domains.RunSubjectDomain,
	fedMarketDataKind vo.MarketDataKindVo,
	requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationRequestDto, error) {
	runnableStrategyScriptDto := runSubjectDomain.ToRunnableDto()

	if strategyScriptID, namesAStrategyScript := runSubjectDomain.NamedStrategyScriptID(); namesAStrategyScript {
		resolved, resolveError := indicatorCalculationApplication.strategyScriptService.ResolveRunnableStrategyScript(
			executionContext, viewerID, strategyScriptID)
		if resolveError != nil {
			return dto.IndicatorCalculationRequestDto{}, resolveError
		}

		marketDataKind, kindError := domains.NewMarketDataKindDomain(resolved.MarketDataKind)
		if kindError != nil {
			return dto.IndicatorCalculationRequestDto{}, kindError
		}
		if mismatchError := marketDataKind.RequireRunnableAs(fedMarketDataKind); mismatchError != nil {
			return dto.IndicatorCalculationRequestDto{}, mismatchError
		}

		runnableStrategyScriptDto = resolved
	}

	requestDto.Script = runnableStrategyScriptDto.Script
	requestDto.ResultType = runnableStrategyScriptDto.ResultType
	requestDto.Parameters = runnableStrategyScriptDto.Parameters

	return requestDto, nil
}
