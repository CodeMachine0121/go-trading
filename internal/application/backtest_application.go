package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
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

// RunBacktest replays whatever this caller is asking to replay: the strategy they
// named, or the algorithm they wrote themselves. The choice is settled by the same
// model a calculation uses, so the two use cases cannot drift on what "one or the
// other" means.
//
// The kind of value is not taken from either, because a replay always reads
// signals — there is nothing for anybody to declare that the replay would honour.
func (backtestApplication *BacktestApplication) RunBacktest(
	executionContext context.Context,
	viewerID uint,
	runSubjectDomain domains.RunSubjectDomain,
	requestDto dto.BacktestRequestDto,
) (dto.BacktestResultDto, error) {
	runnableStrategyDto := runSubjectDomain.ToRunnableDto()

	if strategyID, namesAStrategy := runSubjectDomain.NamedStrategyID(); namesAStrategy {
		resolved, resolveError := backtestApplication.strategyService.ResolveRunnableStrategy(
			executionContext, viewerID, strategyID)
		if resolveError != nil {
			return dto.BacktestResultDto{}, resolveError
		}

		runnableStrategyDto = resolved
	}

	requestDto.Script = runnableStrategyDto.Script
	requestDto.Parameters = runnableStrategyDto.Parameters

	return backtestApplication.backtestService.RunBacktest(executionContext, requestDto)
}
