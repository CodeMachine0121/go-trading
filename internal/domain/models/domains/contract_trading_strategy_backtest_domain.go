package domains

import (
	"fmt"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ContractTradingStrategyBacktestDomain is one contract replay of a trading strategy:
// the signal half is TradingStrategyReplaySourcesDomain, exactly as for a spot replay,
// and the account half is ContractBacktestDomain, exactly as for a contract strategy
// script. What is left here is what makes it a contract trading strategy replay: the
// trading strategy and every source eat contract bars, and the trading mode is the
// trading strategy's to say.
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

	// One set of rules with two sayings of which trading mode it trades by would leave
	// one of them silently ignored, so the replay is not allowed a saying of its own.
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

// ContractBacktest is the replay of the account this trading strategy trades, which is
// what says which bars to read.
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

// ReplayOver turns each bar's several opinions into one and walks the contract account
// over them, adding how many bars the trees conflicted on.
func (strategyBacktestDomain ContractTradingStrategyBacktestDomain) ReplayOver(
	alignment ContractKCandleAlignmentDomain,
	signalsBySource [][]SignalDomain,
	settlements []entities.ContractFundingRateSettlement,
) dto.ContractBacktestResultDto {
	verdicts, conflictedCandleCount := strategyBacktestDomain.sources.Combine(
		len(alignment.buckets), signalsBySource)

	resultDto := strategyBacktestDomain.contractBacktest.ReplayOver(alignment, verdicts, settlements)
	resultDto.Summary.ConflictedCandleCount = conflictedCandleCount

	return resultDto
}
