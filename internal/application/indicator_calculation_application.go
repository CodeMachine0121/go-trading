package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// IndicatorCalculationApplication orchestrates the indicator calculation use case:
// resolve the strategy the caller named, then run it.
//
// The two steps are two domain services, and joining them is this layer's job — a
// domain service does not call another. It is also what makes running somebody
// else's strategy possible without reading it: the script is fetched here and
// handed straight to the calculation, and nothing on the way back out carries it.
type IndicatorCalculationApplication struct {
	strategyService             *service.StrategyService
	indicatorCalculationService *service.IndicatorCalculationService
}

func NewIndicatorCalculationApplication(
	strategyService *service.StrategyService,
	indicatorCalculationService *service.IndicatorCalculationService,
) *IndicatorCalculationApplication {
	return &IndicatorCalculationApplication{
		strategyService:             strategyService,
		indicatorCalculationService: indicatorCalculationService,
	}
}

// CalculateIndicator runs whatever this caller is asking to run: the strategy they
// named, or the algorithm they wrote themselves.
//
// Which of the two it is, is settled before anything else happens — by a model, so
// that "one or the other, never both" is answered in one place for every use case
// that runs something. A named strategy then goes through the three gates, which is
// what lets somebody run another person's published algorithm without being handed
// it; an unsaved one is the caller's own text and needs no gate at all.
func (indicatorCalculationApplication *IndicatorCalculationApplication) CalculateIndicator(
	executionContext context.Context,
	viewerID uint,
	runSubjectDomain domains.RunSubjectDomain,
	requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	runnableStrategyDto := runSubjectDomain.ToRunnableDto()

	if strategyID, namesAStrategy := runSubjectDomain.NamedStrategyID(); namesAStrategy {
		resolved, resolveError := indicatorCalculationApplication.strategyService.ResolveRunnableStrategy(
			executionContext, viewerID, strategyID)
		if resolveError != nil {
			return dto.IndicatorCalculationResultDto{}, resolveError
		}

		runnableStrategyDto = resolved
	}

	requestDto.Script = runnableStrategyDto.Script
	requestDto.ResultType = runnableStrategyDto.ResultType
	requestDto.Parameters = runnableStrategyDto.Parameters

	return indicatorCalculationApplication.indicatorCalculationService.CalculateIndicator(
		executionContext, requestDto)
}
