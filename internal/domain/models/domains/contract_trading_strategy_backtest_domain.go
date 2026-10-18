package domains

import (
	"fmt"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ContractTradingStrategyBacktestDomain combines TradingStrategyReplaySourcesDomain for signals with ContractBacktestDomain for the account, requiring contract-bar sources and the strategy's own trading mode.
type ContractTradingStrategyBacktestDomain struct {
	contractBacktest ContractBacktestDomain
	sources          TradingStrategyReplaySourcesDomain
}

func NewContractTradingStrategyBacktestDomain(
	requestDto dto.ContractTradingStrategyBacktestRequestDto,
	tradingRules ContractTradingRulesDomain,
	maxCandleCount int,
	now time.Time,
) (ContractTradingStrategyBacktestDomain, error) {
	tradingStrategyKind, kindError := NewMarketDataKindDomain(requestDto.TradingStrategyMarketDataKind)
	if kindError != nil {
		return ContractTradingStrategyBacktestDomain{}, fmt.Errorf("%w: %w", ErrBacktestValidation, kindError)
	}
	if tradingStrategyKind.value != vo.MarketDataKindContractKCandle {
		return ContractTradingStrategyBacktestDomain{}, fmt.Errorf(
			"%w: 這份交易策略吃的是%s，不能拿來做合約重演——請改用現貨重演",
			ErrBacktestValidation, tradingStrategyKind.label())
	}

	// The strategy owns the trading mode, so a second one on the request would be silently ignored.
	if strings.TrimSpace(requestDto.TradingMode) != "" {
		return ContractTradingStrategyBacktestDomain{}, BacktestValidationFailure(BacktestTradingModeField,
			"交易模式由交易策略自己決定，重演時不能另外指定——要換交易模式請修改那份交易策略")
	}

	sources, sourcesError := NewTradingStrategyReplaySourcesDomain(
		requestDto.SignalSources, requestDto.BuyCondition, requestDto.SellCondition, tradingStrategyKind)
	if sourcesError != nil {
		return ContractTradingStrategyBacktestDomain{}, sourcesError
	}

	pace := sources.Pace()
	contractBacktest, contractBacktestError := NewContractBacktestDomain(
		requestDto.ToContractBacktestRequestDto(string(pace.Interval().Value())), tradingRules, maxCandleCount, now)
	if contractBacktestError != nil {
		return ContractTradingStrategyBacktestDomain{}, contractBacktestError
	}

	contractBacktest.backtest = contractBacktest.backtest.warmedUpFor(pace.WarmupBarCount())
	if pace.MixesIntervals() {
		if warmupError := contractBacktest.backtest.refusingWarmupBeyond(maxCandleCount); warmupError != nil {
			return ContractTradingStrategyBacktestDomain{}, warmupError
		}
		contractBacktest.backtest = contractBacktest.backtest.readingAlignedTo(pace.CoarsestInterval())
	}

	for sourceIndex := range sources.SourceCount() {
		if reachError := contractBacktest.KCandleQuery().requireOrderFlowReach(
			sources.SourceParameters(sourceIndex)); reachError != nil {
			return ContractTradingStrategyBacktestDomain{}, reachError
		}
	}

	return ContractTradingStrategyBacktestDomain{contractBacktest: contractBacktest, sources: sources}, nil
}

func (strategyBacktestDomain ContractTradingStrategyBacktestDomain) ContractBacktest() ContractBacktestDomain {
	return strategyBacktestDomain.contractBacktest
}

func (strategyBacktestDomain ContractTradingStrategyBacktestDomain) SourceCount() int {
	return strategyBacktestDomain.sources.SourceCount()
}

func (strategyBacktestDomain ContractTradingStrategyBacktestDomain) SourceScript(index int) string {
	return strategyBacktestDomain.sources.SourceScript(index)
}

func (strategyBacktestDomain ContractTradingStrategyBacktestDomain) SourceParameters(
	index int,
) StrategyScriptParametersDomain {
	return strategyBacktestDomain.sources.SourceParameters(index)
}

func (strategyBacktestDomain ContractTradingStrategyBacktestDomain) SourceIsAtPace(index int) bool {
	return strategyBacktestDomain.sources.SourceIsAtPace(index)
}

// SourceAlignment is what a source coarser than the pace reads: its own finished bars from the same
// contract K candles; false when not one of its bars has finished within what was read.
func (strategyBacktestDomain ContractTradingStrategyBacktestDomain) SourceAlignment(
	index int, kCandleContracts []entities.KCandleContract,
) (ContractKCandleAlignmentDomain, bool) {
	return strategyBacktestDomain.contractBacktest.FinishedAlignmentAt(
		strategyBacktestDomain.sources.SourceInterval(index), kCandleContracts)
}

// SignalsAtPace lets a coarser source speak on each pace bar with its last finished bar only.
func (strategyBacktestDomain ContractTradingStrategyBacktestDomain) SignalsAtPace(
	index int,
	sourceAlignment ContractKCandleAlignmentDomain,
	sourceSignals []SignalDomain,
	paceAlignment ContractKCandleAlignmentDomain,
) []SignalDomain {
	return strategyBacktestDomain.sources.Pace().SignalsAtPace(
		strategyBacktestDomain.sources.SourceInterval(index),
		sourceAlignment.openTimes(),
		sourceSignals,
		paceAlignment.openTimes())
}

// ReplayOver combines each bar's source signals into one verdict and replays the contract account over them, recording conflicted bars.
func (strategyBacktestDomain ContractTradingStrategyBacktestDomain) ReplayOver(
	alignment ContractKCandleAlignmentDomain,
	signalsBySource [][]SignalDomain,
	settlements []entities.ContractFundingRateSettlement,
) dto.ContractBacktestResultDto {
	verdicts, conflictedFlags := strategyBacktestDomain.sources.Combine(
		len(alignment.buckets), signalsBySource)

	return strategyBacktestDomain.contractBacktest.ReplayOver(alignment, verdicts, settlements, conflictedFlags)
}
