package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type BacktestApplication struct {
	strategyScriptService   *service.StrategyScriptService
	backtestService         *service.BacktestService
	contractBacktestService *service.ContractBacktestService
}

func NewBacktestApplication(
	strategyScriptService *service.StrategyScriptService,
	backtestService *service.BacktestService,
	contractBacktestService *service.ContractBacktestService,
) *BacktestApplication {
	return &BacktestApplication{
		strategyScriptService:   strategyScriptService,
		backtestService:         backtestService,
		contractBacktestService: contractBacktestService,
	}
}

// RunBacktest replays the named strategy script or the caller's own algorithm on a spot account; the value kind is ignored because a replay always reads signals.
func (backtestApplication *BacktestApplication) RunBacktest(
	executionContext context.Context,
	viewerID uint,
	runSubjectDomain domains.RunSubjectDomain,
	requestDto dto.BacktestRequestDto,
) (dto.BacktestResultDto, error) {
	runnableStrategyScriptDto, resolveError := backtestApplication.resolveReplayable(
		executionContext, viewerID, runSubjectDomain, vo.MarketDataKindKCandle)
	if resolveError != nil {
		return dto.BacktestResultDto{}, resolveError
	}

	requestDto.Script = runnableStrategyScriptDto.Script
	requestDto.Parameters = runnableStrategyScriptDto.Parameters

	return backtestApplication.backtestService.RunBacktest(executionContext, requestDto)
}

func (backtestApplication *BacktestApplication) RunContractBacktest(
	executionContext context.Context,
	viewerID uint,
	runSubjectDomain domains.RunSubjectDomain,
	requestDto dto.ContractBacktestRequestDto,
) (dto.ContractBacktestResultDto, error) {
	runnableStrategyScriptDto, resolveError := backtestApplication.resolveReplayable(
		executionContext, viewerID, runSubjectDomain, vo.MarketDataKindContractKCandle)
	if resolveError != nil {
		return dto.ContractBacktestResultDto{}, resolveError
	}

	requestDto.Script = runnableStrategyScriptDto.Script
	requestDto.Parameters = runnableStrategyScriptDto.Parameters

	return backtestApplication.contractBacktestService.RunContractBacktest(executionContext, requestDto)
}

// resolveReplayable returns the written script or the visible named one, refusing a named script built for the other market kind rather than reporting it as malformed.
func (backtestApplication *BacktestApplication) resolveReplayable(
	executionContext context.Context,
	viewerID uint,
	runSubjectDomain domains.RunSubjectDomain,
	replayedMarketDataKind vo.MarketDataKindVo,
) (dto.RunnableStrategyScriptDto, error) {
	strategyScriptID, namesAStrategyScript := runSubjectDomain.NamedStrategyScriptID()
	if !namesAStrategyScript {
		return runSubjectDomain.ToRunnableDto(), nil
	}

	resolved, resolveError := backtestApplication.strategyScriptService.ResolveRunnableStrategyScript(
		executionContext, viewerID, strategyScriptID)
	if resolveError != nil {
		return dto.RunnableStrategyScriptDto{}, resolveError
	}

	marketDataKind, kindError := domains.NewMarketDataKindDomain(resolved.MarketDataKind)
	if kindError != nil {
		return dto.RunnableStrategyScriptDto{}, kindError
	}
	if mismatchError := marketDataKind.RequireReplayableAs(replayedMarketDataKind); mismatchError != nil {
		return dto.RunnableStrategyScriptDto{}, mismatchError
	}

	return resolved, nil
}
