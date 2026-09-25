package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradingStrategyReplaySourcesDomain is shared by spot and contract replays and judges
// through StrategyBotVerdictDomain, so replays and live rounds reach the same signal.
type TradingStrategyReplaySourcesDomain struct {
	sharedInterval string
	sources        []resolvedSignalSource
	buyCondition   TradingStrategyConditionDomain
	sellCondition  TradingStrategyConditionDomain
}

type resolvedSignalSource struct {
	label      string
	script     string
	parameters StrategyScriptParametersDomain
}

// NewTradingStrategyReplaySourcesDomain checks the shared interval first, then refuses
// sources of the wrong market data kind by name, since sources saved before that rule may
// still be one.
func NewTradingStrategyReplaySourcesDomain(
	signalSources []dto.ResolvedSignalSourceDto,
	buyConditionDto dto.TradingStrategyConditionDto,
	sellConditionDto dto.TradingStrategyConditionDto,
	replayedMarketDataKind MarketDataKindDomain,
) (TradingStrategyReplaySourcesDomain, error) {
	intervals := make([]string, 0, len(signalSources))
	for _, signalSource := range signalSources {
		intervals = append(intervals, signalSource.AggregationInterval)
	}

	sharedInterval, intervalError := NewSharedAggregationIntervalDomain(intervals).Shared()
	if intervalError != nil {
		return TradingStrategyReplaySourcesDomain{}, BacktestValidationFailure(
			BacktestSignalSourcesField, intervalError.Error())
	}

	sources := make([]resolvedSignalSource, 0, len(signalSources))
	labels := make([]string, 0, len(signalSources))
	for _, signalSource := range signalSources {
		label := strings.TrimSpace(signalSource.Label)

		sourceKind, kindError := NewMarketDataKindDomain(signalSource.MarketDataKind)
		if kindError != nil {
			return TradingStrategyReplaySourcesDomain{}, fmt.Errorf("%w: %w", ErrBacktestValidation, kindError)
		}
		if sourceKind.value != replayedMarketDataKind.value {
			return TradingStrategyReplaySourcesDomain{}, BacktestValidationFailure(
				BacktestSignalSourcesField, fmt.Sprintf(
					"信號來源 %q 指名的那支策略腳本吃的是%s，這一種重演走的是%s——"+
						"一份交易策略的每一個信號來源都要吃同一種行情",
					label, sourceKind.label(), replayedMarketDataKind.label()))
		}

		declaredParameters, parametersError := NewStrategyScriptParametersDomain(signalSource.Parameters)
		if parametersError != nil {
			return TradingStrategyReplaySourcesDomain{}, fmt.Errorf(
				"%w: %w", ErrBacktestValidation, parametersError)
		}

		parameters, applyError := declaredParameters.Applying(signalSource.ParameterValues)
		if applyError != nil {
			return TradingStrategyReplaySourcesDomain{}, fmt.Errorf(
				"%w: %w", ErrBacktestValidation, applyError)
		}

		sources = append(sources, resolvedSignalSource{
			label:      label,
			script:     signalSource.Script,
			parameters: parameters,
		})
		labels = append(labels, label)
	}

	buyCondition, buyError := NewTradingStrategyConditionDomain(buyConditionDto, labels)
	if buyError != nil {
		return TradingStrategyReplaySourcesDomain{}, fmt.Errorf("%w（買入條件）", buyError)
	}

	sellCondition, sellError := NewTradingStrategyConditionDomain(sellConditionDto, labels)
	if sellError != nil {
		return TradingStrategyReplaySourcesDomain{}, fmt.Errorf("%w（賣出條件）", sellError)
	}

	return TradingStrategyReplaySourcesDomain{
		sharedInterval: sharedInterval,
		sources:        sources,
		buyCondition:   buyCondition,
		sellCondition:  sellCondition,
	}, nil
}

func (replaySourcesDomain TradingStrategyReplaySourcesDomain) SharedInterval() string {
	return replaySourcesDomain.sharedInterval
}

func (replaySourcesDomain TradingStrategyReplaySourcesDomain) SourceCount() int {
	return len(replaySourcesDomain.sources)
}

func (replaySourcesDomain TradingStrategyReplaySourcesDomain) SourceScript(index int) string {
	return replaySourcesDomain.sources[index].script
}

func (replaySourcesDomain TradingStrategyReplaySourcesDomain) SourceParameters(
	index int,
) StrategyScriptParametersDomain {
	return replaySourcesDomain.sources[index].parameters
}

// Combine returns each bar's signal and whether both trees held; conflicted bars are holds,
// reported per bar so each part of a split replay counts its own.
func (replaySourcesDomain TradingStrategyReplaySourcesDomain) Combine(
	candleCount int, signalsBySource [][]SignalDomain,
) ([]SignalDomain, []bool) {
	verdicts := make([]SignalDomain, 0, candleCount)
	conflictedFlags := make([]bool, 0, candleCount)

	for candleIndex := range candleCount {
		signalsByLabel := make(map[string]vo.SignalVo, len(replaySourcesDomain.sources))
		for sourceIndex, source := range replaySourcesDomain.sources {
			signalsByLabel[source.label] = signalsBySource[sourceIndex][candleIndex].Value()
		}

		verdictDomain := NewStrategyBotVerdictDomain(
			replaySourcesDomain.buyCondition.Holds(signalsByLabel),
			replaySourcesDomain.sellCondition.Holds(signalsByLabel),
			"")

		conflictedFlags = append(conflictedFlags, verdictDomain.IsConflicting())

		signal, hasSignal := verdictDomain.Signal()
		if !hasSignal {
			signal = vo.SignalHold
		}

		verdicts = append(verdicts, NewSignalDomainOf(signal))
	}

	return verdicts, conflictedFlags
}
