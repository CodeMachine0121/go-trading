package application

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// TradingStrategyBacktestApplication joins the trading strategy, strategy script and backtest
// services to replay a strategy over past market data.
type TradingStrategyBacktestApplication struct {
	tradingStrategyService  *service.TradingStrategyService
	strategyScriptService   *service.StrategyScriptService
	backtestService         *service.BacktestService
	contractBacktestService *service.ContractBacktestService
	backtestRunService      *service.BacktestRunService
	// onDemandAllowance bounds a replay someone is waiting on, as the assistant is.
	onDemandAllowance time.Duration
}

func NewTradingStrategyBacktestApplication(
	tradingStrategyService *service.TradingStrategyService,
	strategyScriptService *service.StrategyScriptService,
	backtestService *service.BacktestService,
	contractBacktestService *service.ContractBacktestService,
	backtestRunService *service.BacktestRunService,
	onDemandAllowance time.Duration,
) *TradingStrategyBacktestApplication {
	return &TradingStrategyBacktestApplication{
		tradingStrategyService:  tradingStrategyService,
		strategyScriptService:   strategyScriptService,
		backtestService:         backtestService,
		contractBacktestService: contractBacktestService,
		backtestRunService:      backtestRunService,
		onDemandAllowance:       onDemandAllowance,
	}
}

// SubmitTradingStrategyBacktest queues a spot replay of the strategy once it resolves and passes the replay's own judgement.
func (tradingStrategyBacktestApplication *TradingStrategyBacktestApplication) SubmitTradingStrategyBacktest(
	executionContext context.Context,
	viewerID uint,
	tradingStrategyID uint,
	requestDto dto.TradingStrategyBacktestRequestDto,
) (dto.BacktestRunDto, error) {
	tradingStrategyDto, resolvedSources, authorship, resolveError := tradingStrategyBacktestApplication.
		resolveTradingStrategy(executionContext, viewerID, tradingStrategyID)
	if resolveError != nil {
		return dto.BacktestRunDto{}, resolveError
	}

	requestDto.SignalSources = resolvedSources
	requestDto.BuyCondition = tradingStrategyDto.BuyCondition
	requestDto.SellCondition = tradingStrategyDto.SellCondition
	requestDto.TradingStrategyMarketDataKind = tradingStrategyDto.MarketDataKind
	if validationError := tradingStrategyBacktestApplication.backtestService.ValidateTradingStrategyBacktest(
		requestDto); validationError != nil {
		return dto.BacktestRunDto{}, authorship.AttributeFailure(validationError)
	}

	return tradingStrategyBacktestApplication.backtestRunService.Submit(executionContext, viewerID,
		dto.BacktestRunSubmissionDto{
			Kind:              string(vo.BacktestRunSpotTradingStrategy),
			Request:           dto.BacktestRunRequestDto{SpotTradingStrategy: &requestDto},
			TradingStrategyID: &tradingStrategyID,
			SubjectName:       tradingStrategyDto.Name,
		})
}

// SubmitContractTradingStrategyBacktest is the contract-account counterpart of SubmitTradingStrategyBacktest.
func (tradingStrategyBacktestApplication *TradingStrategyBacktestApplication) SubmitContractTradingStrategyBacktest(
	executionContext context.Context,
	viewerID uint,
	tradingStrategyID uint,
	requestDto dto.ContractTradingStrategyBacktestRequestDto,
) (dto.BacktestRunDto, error) {
	tradingStrategyDto, resolvedSources, authorship, resolveError := tradingStrategyBacktestApplication.
		resolveTradingStrategy(executionContext, viewerID, tradingStrategyID)
	if resolveError != nil {
		return dto.BacktestRunDto{}, resolveError
	}

	requestDto.SignalSources = resolvedSources
	requestDto.BuyCondition = tradingStrategyDto.BuyCondition
	requestDto.SellCondition = tradingStrategyDto.SellCondition
	requestDto.TradingStrategyMarketDataKind = tradingStrategyDto.MarketDataKind
	requestDto.TradingStrategyTradingMode = tradingStrategyDto.TradingMode
	if validationError := tradingStrategyBacktestApplication.contractBacktestService.
		ValidateContractTradingStrategyBacktest(executionContext, requestDto); validationError != nil {
		return dto.BacktestRunDto{}, authorship.AttributeFailure(validationError)
	}

	return tradingStrategyBacktestApplication.backtestRunService.Submit(executionContext, viewerID,
		dto.BacktestRunSubmissionDto{
			Kind:              string(vo.BacktestRunContractTradingStrategy),
			Request:           dto.BacktestRunRequestDto{ContractTradingStrategy: &requestDto},
			TradingStrategyID: &tradingStrategyID,
			SubjectName:       tradingStrategyDto.Name,
		})
}

// RunTradingStrategyBacktest replays the strategy on the spot, for callers that need the report now; a strategy that is not
// this person's fails like a missing one.
func (tradingStrategyBacktestApplication *TradingStrategyBacktestApplication) RunTradingStrategyBacktest(
	executionContext context.Context,
	viewerID uint,
	tradingStrategyID uint,
	requestDto dto.TradingStrategyBacktestRequestDto,
) (dto.BacktestResultDto, error) {
	tradingStrategyDto, resolvedSources, authorship, resolveError := tradingStrategyBacktestApplication.
		resolveTradingStrategy(executionContext, viewerID, tradingStrategyID)
	if resolveError != nil {
		return dto.BacktestResultDto{}, resolveError
	}

	requestDto.SignalSources = resolvedSources
	requestDto.BuyCondition = tradingStrategyDto.BuyCondition
	requestDto.SellCondition = tradingStrategyDto.SellCondition
	requestDto.TradingStrategyMarketDataKind = tradingStrategyDto.MarketDataKind
	requestDto.TimeAllowance = tradingStrategyBacktestApplication.onDemandAllowance

	resultDto, replayError := tradingStrategyBacktestApplication.backtestService.RunTradingStrategyBacktest(
		executionContext, requestDto, nil)

	return resultDto, authorship.AttributeFailure(replayError)
}

// resolveTradingStrategy reads the viewer's strategy with every source's script; a strategy or script the viewer may not read fails like a missing one.
func (tradingStrategyBacktestApplication *TradingStrategyBacktestApplication) resolveTradingStrategy(
	executionContext context.Context, viewerID uint, tradingStrategyID uint,
) (dto.TradingStrategyDto, []dto.ResolvedSignalSourceDto, domains.StrategyScriptAuthorshipDomain, error) {
	tradingStrategyDto, findError := tradingStrategyBacktestApplication.tradingStrategyService.
		GetTradingStrategy(executionContext, viewerID, tradingStrategyID)
	if findError != nil {
		return dto.TradingStrategyDto{}, nil, domains.StrategyScriptAuthorshipDomain{}, findError
	}

	resolvedSources, authorship, resolveError := tradingStrategyBacktestApplication.strategyScriptService.
		ResolveSignalSources(executionContext, viewerID, tradingStrategyDto)

	return tradingStrategyDto, resolvedSources, authorship, resolveError
}
