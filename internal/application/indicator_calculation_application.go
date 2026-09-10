package application

import (
	"context"

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

// CalculateIndicator runs the strategy this caller named over the stretch of market
// the request describes.
//
// The request arrives without an algorithm, and that is the point: a caller who
// could send one would be a caller who already had it. What it may still send is
// what the knobs are worth this time — those are the caller's, and they are used
// for this run and never written back.
func (indicatorCalculationApplication *IndicatorCalculationApplication) CalculateIndicator(
	executionContext context.Context, viewerID uint, strategyID uint, requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	runnableStrategyDto, resolveError := indicatorCalculationApplication.strategyService.ResolveRunnableStrategy(
		executionContext, viewerID, strategyID)
	if resolveError != nil {
		return dto.IndicatorCalculationResultDto{}, resolveError
	}

	requestDto.Script = runnableStrategyDto.Script
	requestDto.ResultType = runnableStrategyDto.ResultType
	requestDto.Parameters = runnableStrategyDto.Parameters

	return indicatorCalculationApplication.indicatorCalculationService.CalculateIndicator(
		executionContext, requestDto)
}

// CalculateAdHocIndicator runs a script that was never saved.
//
// It exists for the assistant, which composes an algorithm on the spot when asked
// something no saved strategy answers. That is not a hole in "a script never comes
// from outside": the script here was written by the assistant on this person's
// behalf, and it is theirs — there is nobody it is being hidden from.
//
// Nothing over HTTP reaches this. The calculation endpoint names a strategy, and
// that is what keeps somebody else's algorithm out of reach.
func (indicatorCalculationApplication *IndicatorCalculationApplication) CalculateAdHocIndicator(
	executionContext context.Context, requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	return indicatorCalculationApplication.indicatorCalculationService.CalculateIndicator(
		executionContext, requestDto)
}
