package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// BacktestApplication orchestrates the strategy backtest use case: resolve the
// strategy the caller named, then replay it.
//
// It joins the same two domain services an indicator calculation does, and for the
// same reason — a script that arrives from outside is a script the caller already
// holds.
type BacktestApplication struct {
	strategyService *service.StrategyService
	backtestService *service.BacktestService
}

func NewBacktestApplication(
	strategyService *service.StrategyService,
	backtestService *service.BacktestService,
) *BacktestApplication {
	return &BacktestApplication{strategyService: strategyService, backtestService: backtestService}
}

// RunBacktest replays the strategy this caller named over the stretch of market the
// request describes.
//
// The kind of value is not taken from the strategy here, because a replay always
// reads signals — there is nothing for the strategy to declare that the replay
// would honour.
func (backtestApplication *BacktestApplication) RunBacktest(
	executionContext context.Context, viewerID uint, strategyID uint, requestDto dto.BacktestRequestDto,
) (dto.BacktestResultDto, error) {
	runnableStrategyDto, resolveError := backtestApplication.strategyService.ResolveRunnableStrategy(
		executionContext, viewerID, strategyID)
	if resolveError != nil {
		return dto.BacktestResultDto{}, resolveError
	}

	requestDto.Script = runnableStrategyDto.Script
	requestDto.Parameters = runnableStrategyDto.Parameters

	return backtestApplication.backtestService.RunBacktest(executionContext, requestDto)
}
