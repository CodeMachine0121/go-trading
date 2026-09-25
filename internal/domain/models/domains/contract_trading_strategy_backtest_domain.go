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

	contractBacktest, contractBacktestError := NewContractBacktestDomain(
		requestDto.ToContractBacktestRequestDto(sources.SharedInterval()), tradingRules, maxCandleCount, now)
	if contractBacktestError != nil {
		return ContractTradingStrategyBacktestDomain{}, contractBacktestError
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
