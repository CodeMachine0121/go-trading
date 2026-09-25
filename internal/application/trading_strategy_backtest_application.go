package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// TradingStrategyBacktestApplication joins the trading strategy, strategy script and backtest
// services to replay a strategy over past market data.
type TradingStrategyBacktestApplication struct {
	tradingStrategyService  *service.TradingStrategyService
	strategyScriptService   *service.StrategyScriptService
	backtestService         *service.BacktestService
	contractBacktestService *service.ContractBacktestService
}

func NewTradingStrategyBacktestApplication(
	tradingStrategyService *service.TradingStrategyService,
	strategyScriptService *service.StrategyScriptService,
	backtestService *service.BacktestService,
	contractBacktestService *service.ContractBacktestService,
) *TradingStrategyBacktestApplication {
	return &TradingStrategyBacktestApplication{
		tradingStrategyService:  tradingStrategyService,
		strategyScriptService:   strategyScriptService,
		backtestService:         backtestService,
		contractBacktestService: contractBacktestService,
	}
}

// RunTradingStrategyBacktest replays the strategy without storing anything; a strategy that is not
// this person's fails like a missing one.
func (tradingStrategyBacktestApplication *TradingStrategyBacktestApplication) RunTradingStrategyBacktest(
	executionContext context.Context,
	viewerID uint,
	tradingStrategyID uint,
	requestDto dto.TradingStrategyBacktestRequestDto,
) (dto.BacktestResultDto, error) {
	tradingStrategyDto, findError := tradingStrategyBacktestApplication.tradingStrategyService.
		GetTradingStrategy(executionContext, viewerID, tradingStrategyID)
	if findError != nil {
		return dto.BacktestResultDto{}, findError
	}

	resolvedSources, resolveError := tradingStrategyBacktestApplication.resolveSignalSources(
		executionContext, viewerID, tradingStrategyDto)
	if resolveError != nil {
		return dto.BacktestResultDto{}, resolveError
	}

	requestDto.SignalSources = resolvedSources
	requestDto.BuyCondition = tradingStrategyDto.BuyCondition
	requestDto.SellCondition = tradingStrategyDto.SellCondition
	requestDto.TradingStrategyMarketDataKind = tradingStrategyDto.MarketDataKind

	return tradingStrategyBacktestApplication.backtestService.RunTradingStrategyBacktest(
		executionContext, requestDto)
}

// RunContractTradingStrategyBacktest is the contract-account counterpart of
// RunTradingStrategyBacktest, with the same gates.
func (tradingStrategyBacktestApplication *TradingStrategyBacktestApplication) RunContractTradingStrategyBacktest(
	executionContext context.Context,
	viewerID uint,
	tradingStrategyID uint,
	requestDto dto.ContractTradingStrategyBacktestRequestDto,
) (dto.ContractBacktestResultDto, error) {
	tradingStrategyDto, findError := tradingStrategyBacktestApplication.tradingStrategyService.
		GetTradingStrategy(executionContext, viewerID, tradingStrategyID)
	if findError != nil {
		return dto.ContractBacktestResultDto{}, findError
	}

	resolvedSources, resolveError := tradingStrategyBacktestApplication.resolveSignalSources(
		executionContext, viewerID, tradingStrategyDto)
	if resolveError != nil {
		return dto.ContractBacktestResultDto{}, resolveError
	}

	requestDto.SignalSources = resolvedSources
	requestDto.BuyCondition = tradingStrategyDto.BuyCondition
	requestDto.SellCondition = tradingStrategyDto.SellCondition
	requestDto.TradingStrategyMarketDataKind = tradingStrategyDto.MarketDataKind
	requestDto.TradingStrategyTradingMode = tradingStrategyDto.TradingMode

	return tradingStrategyBacktestApplication.contractBacktestService.RunContractTradingStrategyBacktest(
		executionContext, requestDto)
}

// resolveSignalSources fetches each source's script and re-checks access, since a script can be
// deleted or withdrawn after the strategy was saved.
func (tradingStrategyBacktestApplication *TradingStrategyBacktestApplication) resolveSignalSources(
	executionContext context.Context, viewerID uint, tradingStrategyDto dto.TradingStrategyDto,
) ([]dto.ResolvedSignalSourceDto, error) {
	resolvedSources := make([]dto.ResolvedSignalSourceDto, 0, len(tradingStrategyDto.SignalSources))

	for _, signalSource := range tradingStrategyDto.SignalSources {
		runnableStrategyScript, resolveError := tradingStrategyBacktestApplication.strategyScriptService.
			ResolveRunnableStrategyScript(executionContext, viewerID, signalSource.StrategyScriptID)
		if resolveError != nil {
			return nil, resolveError
		}

		resolvedSources = append(resolvedSources, dto.ResolvedSignalSourceDto{
			Label:               signalSource.Label,
			AggregationInterval: signalSource.AggregationInterval,
			Script:              runnableStrategyScript.Script,
			MarketDataKind:      runnableStrategyScript.MarketDataKind,
			Parameters:          runnableStrategyScript.Parameters,
			ParameterValues:     signalSource.ParameterValues,
		})
	}

	return resolvedSources, nil
}
