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
	pace          ReplayPaceDomain
	sources       []resolvedSignalSource
	buyCondition  TradingStrategyConditionDomain
	sellCondition TradingStrategyConditionDomain
}

type resolvedSignalSource struct {
	label      string
	script     string
	interval   AggregationIntervalDomain
	parameters StrategyScriptParametersDomain
}

// NewTradingStrategyReplaySourcesDomain refuses sources of the wrong market data kind by name,
// since sources saved before that rule may still be one.
func NewTradingStrategyReplaySourcesDomain(
	signalSources []dto.ResolvedSignalSourceDto,
	buyConditionDto dto.TradingStrategyConditionDto,
	sellConditionDto dto.TradingStrategyConditionDto,
	replayedMarketDataKind MarketDataKindDomain,
) (TradingStrategyReplaySourcesDomain, error) {
	if len(signalSources) == 0 {
		return TradingStrategyReplaySourcesDomain{}, BacktestValidationFailure(
			BacktestSignalSourcesField, "這一份交易策略沒有任何信號來源")
	}

	sources := make([]resolvedSignalSource, 0, len(signalSources))
	labels := make([]string, 0, len(signalSources))
	for _, signalSource := range signalSources {
		label := strings.TrimSpace(signalSource.Label)

		interval, intervalError := NewAggregationIntervalDomain(signalSource.AggregationInterval)
		if intervalError != nil {
			return TradingStrategyReplaySourcesDomain{}, BacktestValidationFailure(
				BacktestSignalSourcesField, fmt.Sprintf("信號來源 %q 的彙總刻度不對：%s", label, intervalError.Error()))
		}

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

		if kindError := parameters.RequireMarketDataKind(replayedMarketDataKind); kindError != nil {
			return TradingStrategyReplaySourcesDomain{}, fmt.Errorf("%w: %w", ErrBacktestValidation, kindError)
		}

		sources = append(sources, resolvedSignalSource{
			label:      label,
			script:     signalSource.Script,
			interval:   interval,
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

	intervals := make([]AggregationIntervalDomain, 0, len(sources))
	lookbackCounts := make([]int, 0, len(sources))
	for _, source := range sources {
		intervals = append(intervals, source.interval)
		lookbackCounts = append(lookbackCounts, source.parameters.MaximumLookbackCount())
	}

	return TradingStrategyReplaySourcesDomain{
		pace:          NewReplayPaceDomain(intervals, lookbackCounts),
		sources:       sources,
		buyCondition:  buyCondition,
		sellCondition: sellCondition,
	}, nil
}

func (replaySourcesDomain TradingStrategyReplaySourcesDomain) Pace() ReplayPaceDomain {
	return replaySourcesDomain.pace
}

func (replaySourcesDomain TradingStrategyReplaySourcesDomain) SourceInterval(index int) AggregationIntervalDomain {
	return replaySourcesDomain.sources[index].interval
}

// SourceIsAtPace tells whether a source reads the pace bars themselves, so its signals need no alignment.
func (replaySourcesDomain TradingStrategyReplaySourcesDomain) SourceIsAtPace(index int) bool {
	return replaySourcesDomain.sources[index].interval.duration == replaySourcesDomain.pace.interval.duration
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
