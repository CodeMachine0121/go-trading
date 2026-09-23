package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// IndicatorCalculationApplication orchestrates the indicator calculation use case:
// resolve the strategy script the caller named, then run it.
//
// The two steps are two domain services, and joining them is this layer's job — a
// domain service does not call another. It is also what makes running somebody
// else's strategy script possible without reading it: the script is fetched here and
// handed straight to the calculation, and nothing on the way back out carries it.
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

// CalculateIndicator runs whatever this caller is asking to run over spot K candles:
// the strategy script they named, or the algorithm they wrote themselves.
//
// Which of the two it is, is settled before anything else happens — by a model, so
// that "one or the other, never both" is answered in one place for every use case
// that runs something. A named strategy script then goes through the three gates, which is
// what lets somebody run another person's published algorithm without being handed
// it; an unsaved one is the caller's own text and needs no gate at all.
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

// CalculateContractIndicator is CalculateIndicator over perpetual contract bars: the
// same choice between a named strategy script and the caller's own algorithm, the
// same three gates, and a calculation that feeds the script contract bars instead.
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

// resolveRunnable fills the request in with the algorithm that is actually going to
// run, for a calculation that feeds the given kind of market.
//
// A named strategy script is resolved through the three gates, and is then refused if
// it eats the other kind of market: handed over anyway, its entry point would not
// take what it is fed, and the caller would read "the script is written wrong" about
// a script written exactly right for somewhere else. The caller's own algorithm is not
// asked — carried to this calculation, it is by that act a script for this kind of
// market, and an entry point that says otherwise is reported as the script's mistake.
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
