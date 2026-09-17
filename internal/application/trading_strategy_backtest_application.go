package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// TradingStrategyBacktestApplication orchestrates replaying one of this person's
// trading strategies over a stretch of market that has already happened.
//
// It joins three domain services, which is this layer's job and not theirs. A replay
// needs the rules (whose they are is the trading strategy's question), the scripts
// those rules name (whether they may be read is the strategy script rules' question),
// and the candles and the account (the replay's). None of the three knows the others
// exist.
type TradingStrategyBacktestApplication struct {
	tradingStrategyService *service.TradingStrategyService
	strategyScriptService  *service.StrategyScriptService
	backtestService        *service.BacktestService
}

func NewTradingStrategyBacktestApplication(
	tradingStrategyService *service.TradingStrategyService,
	strategyScriptService *service.StrategyScriptService,
	backtestService *service.BacktestService,
) *TradingStrategyBacktestApplication {
	return &TradingStrategyBacktestApplication{
		tradingStrategyService: tradingStrategyService,
		strategyScriptService:  strategyScriptService,
		backtestService:        backtestService,
	}
}

// RunTradingStrategyBacktest replays the named trading strategy and hands back the
// report card, the finished round trips and the equity curve. Nothing is stored.
//
// Naming one that is not this person's fails with the same sentence as naming one
// that does not exist, which is what stops the field becoming a way to probe for
// other people's trading strategies.
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
	// Read off the trading strategy rather than taken from whoever asked — and there
	// is nowhere for them to say it, because a sell means what these rules say it
	// means. Somebody replaying rules written for an account that cannot short would
	// otherwise have to remember to say so every single time, under a default that is
	// wrong for them.
	requestDto.TradingMode = tradingStrategyDto.TradingMode

	return tradingStrategyBacktestApplication.backtestService.RunTradingStrategyBacktest(
		executionContext, requestDto)
}

// resolveSignalSources fetches the script behind every source, the one thing a replay
// cannot get for itself.
//
// It is also the gate. A source naming a strategy script that is not this person's
// and not on the marketplace fails here with the same sentence as naming one that
// does not exist — the same rule that applies when the trading strategy was saved,
// asked again now because a script can be deleted in between.
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
			Parameters:          runnableStrategyScript.Parameters,
			ParameterValues:     signalSource.ParameterValues,
		})
	}

	return resolvedSources, nil
}
