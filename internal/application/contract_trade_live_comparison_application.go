package application

import (
	"context"
	"errors"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// ContractTradeLiveComparisonApplication replays a strategy over the stretches its live trades covered, so the person sees whether live results drift from the replay.
type ContractTradeLiveComparisonApplication struct {
	contractTradeJournalService *service.ContractTradeJournalService
	tradingStrategyService      *service.TradingStrategyService
	strategyScriptService       *service.StrategyScriptService
	contractBacktestService     *service.ContractBacktestService
	// onDemandAllowance bounds each replay, since the person is waiting on the answer.
	onDemandAllowance time.Duration
}

func NewContractTradeLiveComparisonApplication(
	contractTradeJournalService *service.ContractTradeJournalService,
	tradingStrategyService *service.TradingStrategyService,
	strategyScriptService *service.StrategyScriptService,
	contractBacktestService *service.ContractBacktestService,
	onDemandAllowance time.Duration,
) *ContractTradeLiveComparisonApplication {
	return &ContractTradeLiveComparisonApplication{
		contractTradeJournalService: contractTradeJournalService,
		tradingStrategyService:      tradingStrategyService,
		strategyScriptService:       strategyScriptService,
		contractBacktestService:     contractBacktestService,
		onDemandAllowance:           onDemandAllowance,
	}
}

// CompareWithBacktest replays one symbol at a time so a slow replay never multiplies; a failed replay costs only its own row.
func (comparisonApplication *ContractTradeLiveComparisonApplication) CompareWithBacktest(
	executionContext context.Context, viewerID uint, tradingStrategyID uint,
) (dto.ContractTradeLiveComparisonDto, error) {
	plan, planError := comparisonApplication.contractTradeJournalService.PlanLiveComparison(
		executionContext, viewerID, tradingStrategyID)
	if planError != nil {
		return dto.ContractTradeLiveComparisonDto{}, planError
	}

	groups := plan.Groups
	comparisonDto := dto.ContractTradeLiveComparisonDto{
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
		comparisonDto.Rows = comparisonApplication.contractTradeJournalService.
			ComposeLiveComparisonForDeletedTradingStrategy(groups)

		return comparisonDto, nil
	}
	if strategyError != nil {
		return dto.ContractTradeLiveComparisonDto{}, strategyError
	}
	comparisonDto.TradingStrategyName = tradingStrategyDto.Name

	resolvedSources, authorship, resolveError := comparisonApplication.strategyScriptService.ResolveSignalSources(
		executionContext, viewerID, tradingStrategyDto)

	attempts := make([]dto.ContractTradeBacktestAttemptDto, 0, len(groups))
	for _, group := range groups {
		if resolveError != nil {
			attempts = append(attempts, dto.ContractTradeBacktestAttemptDto{FailureReason: resolveError.Error()})
			continue
		}

		requestDto := group.BacktestRequest
		requestDto.SignalSources = resolvedSources
		requestDto.BuyCondition = tradingStrategyDto.BuyCondition
		requestDto.SellCondition = tradingStrategyDto.SellCondition
		requestDto.TradingStrategyMarketDataKind = tradingStrategyDto.MarketDataKind
		requestDto.TradingStrategyTradingMode = tradingStrategyDto.TradingMode
		requestDto.TimeAllowance = comparisonApplication.onDemandAllowance

		resultDto, replayError := comparisonApplication.contractBacktestService.RunContractTradingStrategyBacktest(
			executionContext, requestDto, nil)
		if replayError != nil {
			attempts = append(attempts, dto.ContractTradeBacktestAttemptDto{
				FailureReason: authorship.AttributeFailure(replayError).Error(),
			})
			continue
		}
		attempts = append(attempts, dto.ContractTradeBacktestAttemptDto{Result: resultDto})
	}

	comparisonDto.Rows = comparisonApplication.contractTradeJournalService.ComposeLiveComparison(groups, attempts)

	return comparisonDto, nil
}
