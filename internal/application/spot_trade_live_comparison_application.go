package application

import (
	"context"
	"errors"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// SpotTradeLiveComparisonApplication replays a spot strategy over the stretches its live trades covered, so the person sees whether live results drift from the replay.
type SpotTradeLiveComparisonApplication struct {
	spotTradeJournalService *service.SpotTradeJournalService
	tradingStrategyService  *service.TradingStrategyService
	strategyScriptService   *service.StrategyScriptService
	backtestService         *service.BacktestService
}

func NewSpotTradeLiveComparisonApplication(
	spotTradeJournalService *service.SpotTradeJournalService,
	tradingStrategyService *service.TradingStrategyService,
	strategyScriptService *service.StrategyScriptService,
	backtestService *service.BacktestService,
) *SpotTradeLiveComparisonApplication {
	return &SpotTradeLiveComparisonApplication{
		spotTradeJournalService: spotTradeJournalService,
		tradingStrategyService:  tradingStrategyService,
		strategyScriptService:   strategyScriptService,
		backtestService:         backtestService,
	}
}

// CompareWithBacktest replays one symbol at a time so a slow replay never multiplies; a failed replay costs only its own row.
func (comparisonApplication *SpotTradeLiveComparisonApplication) CompareWithBacktest(
	executionContext context.Context, viewerID uint, tradingStrategyID uint,
) (dto.SpotTradeLiveComparisonDto, error) {
	plan, planError := comparisonApplication.spotTradeJournalService.PlanLiveComparison(
		executionContext, viewerID, tradingStrategyID)
	if planError != nil {
		return dto.SpotTradeLiveComparisonDto{}, planError
	}

	groups := plan.Groups
	comparisonDto := dto.SpotTradeLiveComparisonDto{
		TradingStrategyID:              tradingStrategyID,
		NoClosedTrades:                 len(groups) == 0,
		AverageEntrySlippagePercentage: plan.AverageEntrySlippagePercentage,
		EntrySlippageTradeCount:        plan.EntrySlippageTradeCount,
	}

	tradingStrategyDto, strategyError := comparisonApplication.tradingStrategyService.GetTradingStrategy(
		executionContext, viewerID, tradingStrategyID)
	// Live trades still pointing at the strategy prove it was the person's, so a missing one was deleted.
	if errors.Is(strategyError, domains.ErrTradingStrategyNotFound) && len(groups) > 0 {
		comparisonDto.TradingStrategyDeleted = true
		comparisonDto.Rows = comparisonApplication.spotTradeJournalService.
			ComposeLiveComparisonForDeletedTradingStrategy(groups)

		return comparisonDto, nil
	}
	if strategyError != nil {
		return dto.SpotTradeLiveComparisonDto{}, strategyError
	}
	comparisonDto.TradingStrategyName = tradingStrategyDto.Name

	resolvedSources, authorship, resolveError := comparisonApplication.strategyScriptService.ResolveSignalSources(
		executionContext, viewerID, tradingStrategyDto)

	attempts := make([]dto.SpotTradeBacktestAttemptDto, 0, len(groups))
	for _, group := range groups {
		if resolveError != nil {
			attempts = append(attempts, dto.SpotTradeBacktestAttemptDto{FailureReason: resolveError.Error()})
			continue
		}

		requestDto := group.BacktestRequest
		requestDto.SignalSources = resolvedSources
		requestDto.BuyCondition = tradingStrategyDto.BuyCondition
		requestDto.SellCondition = tradingStrategyDto.SellCondition
		requestDto.TradingStrategyMarketDataKind = tradingStrategyDto.MarketDataKind

		resultDto, replayError := comparisonApplication.backtestService.RunTradingStrategyBacktest(
			executionContext, requestDto)
		if replayError != nil {
			attempts = append(attempts, dto.SpotTradeBacktestAttemptDto{
				FailureReason: authorship.AttributeFailure(replayError).Error(),
			})
			continue
		}
		attempts = append(attempts, dto.SpotTradeBacktestAttemptDto{Result: resultDto})
	}

	comparisonDto.Rows = comparisonApplication.spotTradeJournalService.ComposeLiveComparison(groups, attempts)

	return comparisonDto, nil
}
