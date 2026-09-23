package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// BacktestApplication orchestrates the strategy script backtest use cases: resolve the
// strategy script the caller named, then replay it — on a spot account, or on a
// contract account.
//
// It joins the same two kinds of domain service an indicator calculation does, and for
// the same reason — a script that arrives from outside is a script the caller already
// holds.
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

// RunBacktest replays whatever this caller is asking to replay on a spot account: the
// strategy script they named, or the algorithm they wrote themselves. The choice is
// settled by the same model a calculation uses, so the two use cases cannot drift on
// what "one or the other" means.
//
// The kind of value is not taken from either, because a replay always reads
// signals — there is nothing for anybody to declare that the replay would honour.
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

// RunContractBacktest replays a contract strategy script, named or written, on a
// contract account.
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

// resolveReplayable is the script a replay over that kind of market will run: the one
// written in the request, or the named one — which has to be visible to this person and
// has to eat that kind of market. A script eating the other kind is refused for what it
// is, rather than being fed a shape its entry point does not take and reported as
// written wrong.
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
