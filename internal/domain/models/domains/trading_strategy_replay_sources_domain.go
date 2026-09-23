package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradingStrategyReplaySourcesDomain is the signal half of replaying a trading
// strategy: its sources, each with its script and knobs settled, the coarseness they
// all share, and the two condition trees that turn one bar's several opinions into
// one.
//
// It is one model shared by the spot and the contract replay of a trading strategy,
// because the two must reach the same signal on the same bar from the same opinions —
// a contract trading strategy with a single source must replay exactly as that one
// script does. What they do with the signal afterwards is each replay's own business.
//
// The judgement itself goes through StrategyBotVerdictDomain, the model a live round
// uses, for the reason it always has: a rehearsal and the thing it rehearses must not
// be two separately written judgements.
type TradingStrategyReplaySourcesDomain struct {
	sharedInterval string
	sources        []resolvedSignalSource
	buyCondition   TradingStrategyConditionDomain
	sellCondition  TradingStrategyConditionDomain
}

// resolvedSignalSource is one source as a replay will run it: what to call it in the
// conditions, the script, and its knobs already settled.
type resolvedSignalSource struct {
	label      string
	script     string
	parameters StrategyScriptParametersDomain
}

// NewTradingStrategyReplaySourcesDomain settles the sources for a replay that walks
// over bars of that kind of market. The shared coarseness is asked first, because it
// is the one rule that decides which bars every other rule is about; a source eating
// the other kind of market is refused by name, because a source saved before that
// rule existed can still be one.
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

// SharedInterval is the coarseness every source reads, which is the one the replay
// walks at.
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

// Combine turns every source's opinion about each bar into that bar's one signal, and
// counts the bars on which both trees held. Such a bar is a hold — choosing a side for
// the strategy would be trading on an opinion it never had — and it is counted,
// because a strategy conflicting on most bars barely trades and reads as steady.
func (replaySourcesDomain TradingStrategyReplaySourcesDomain) Combine(
	candleCount int, signalsBySource [][]SignalDomain,
) ([]SignalDomain, int) {
	verdicts := make([]SignalDomain, 0, candleCount)
	conflictedCandleCount := 0

	for candleIndex := range candleCount {
		signalsByLabel := make(map[string]vo.SignalVo, len(replaySourcesDomain.sources))
		for sourceIndex, source := range replaySourcesDomain.sources {
			signalsByLabel[source.label] = signalsBySource[sourceIndex][candleIndex].Value()
		}

		verdictDomain := NewStrategyBotVerdictDomain(
			replaySourcesDomain.buyCondition.Holds(signalsByLabel),
			replaySourcesDomain.sellCondition.Holds(signalsByLabel),
			"")

		if verdictDomain.IsConflicting() {
			conflictedCandleCount++
		}

		signal, hasSignal := verdictDomain.Signal()
		if !hasSignal {
			signal = vo.SignalHold
		}

		verdicts = append(verdicts, NewSignalDomainOf(signal))
	}

	return verdicts, conflictedCandleCount
}
