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
	backtestRunService      *service.BacktestRunService
}

func NewBacktestApplication(
	strategyScriptService *service.StrategyScriptService,
	backtestService *service.BacktestService,
	contractBacktestService *service.ContractBacktestService,
	backtestRunService *service.BacktestRunService,
) *BacktestApplication {
	return &BacktestApplication{
		strategyScriptService:   strategyScriptService,
		backtestService:         backtestService,
		contractBacktestService: contractBacktestService,
		backtestRunService:      backtestRunService,
	}
}

// SubmitBacktest queues a spot replay of the named strategy script or the caller's own algorithm; the value kind is ignored because a replay always reads signals.
func (backtestApplication *BacktestApplication) SubmitBacktest(
	executionContext context.Context,
	viewerID uint,
	runSubjectDomain domains.RunSubjectDomain,
	requestDto dto.BacktestRequestDto,
) (dto.BacktestRunDto, error) {
	runnableStrategyScriptDto, resolveError := backtestApplication.resolveReplayable(
		executionContext, viewerID, runSubjectDomain, vo.MarketDataKindKCandle)
	if resolveError != nil {
		return dto.BacktestRunDto{}, resolveError
	}

	requestDto.Script = runnableStrategyScriptDto.Script
	requestDto.Parameters = runnableStrategyScriptDto.Parameters
	if validationError := backtestApplication.backtestService.ValidateBacktest(requestDto); validationError != nil {
		return dto.BacktestRunDto{}, validationError
	}

	strategyScriptID, subjectName := backtestApplication.subjectOf(executionContext, viewerID, runSubjectDomain)

	return backtestApplication.backtestRunService.Submit(executionContext, viewerID, dto.BacktestRunSubmissionDto{
		Kind:             string(vo.BacktestRunSpotScript),
		Request:          dto.BacktestRunRequestDto{SpotScript: &requestDto},
		StrategyScriptID: strategyScriptID,
		SubjectName:      subjectName,
	})
}

func (backtestApplication *BacktestApplication) SubmitContractBacktest(
	executionContext context.Context,
	viewerID uint,
	runSubjectDomain domains.RunSubjectDomain,
	requestDto dto.ContractBacktestRequestDto,
) (dto.BacktestRunDto, error) {
	runnableStrategyScriptDto, resolveError := backtestApplication.resolveReplayable(
		executionContext, viewerID, runSubjectDomain, vo.MarketDataKindContractKCandle)
	if resolveError != nil {
		return dto.BacktestRunDto{}, resolveError
	}

	requestDto.Script = runnableStrategyScriptDto.Script
	requestDto.Parameters = runnableStrategyScriptDto.Parameters
	if validationError := backtestApplication.contractBacktestService.ValidateContractBacktest(
		executionContext, requestDto); validationError != nil {
		return dto.BacktestRunDto{}, validationError
	}

	strategyScriptID, subjectName := backtestApplication.subjectOf(executionContext, viewerID, runSubjectDomain)

	return backtestApplication.backtestRunService.Submit(executionContext, viewerID, dto.BacktestRunSubmissionDto{
		Kind:             string(vo.BacktestRunContractScript),
		Request:          dto.BacktestRunRequestDto{ContractScript: &requestDto},
		StrategyScriptID: strategyScriptID,
		SubjectName:      subjectName,
	})
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

// subjectOf names the run after the script it replays; an unsaved algorithm or a script the viewer cannot read by name has no name to give.
func (backtestApplication *BacktestApplication) subjectOf(
	executionContext context.Context, viewerID uint, runSubjectDomain domains.RunSubjectDomain,
) (*uint, string) {
	strategyScriptID, namesAStrategyScript := runSubjectDomain.NamedStrategyScriptID()
	if !namesAStrategyScript {
		return nil, ""
	}

	strategyScriptDto, readError := backtestApplication.strategyScriptService.GetStrategyScript(
		executionContext, viewerID, strategyScriptID)
	if readError != nil {
		return &strategyScriptID, ""
	}

	return &strategyScriptID, strategyScriptDto.Name
}
