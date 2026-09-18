package domains

import (
	"fmt"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradingStrategyBacktestDomain is one replay of a trading strategy, and the one rule
// that is new to it: every signal source has to read the same coarseness.
//
// Everything else about a replay — which stretch, how much capital, how much each
// opening stakes, which candles count as finished, how the account is walked — is
// delegated to BacktestDomain untouched. What this model adds is the step in the
// middle: several sources each have an opinion about the same candle, and two
// condition trees turn those into one.
//
// That last step goes through StrategyBotVerdictDomain, the same model a live round
// uses. A replay and the bot it is a rehearsal for must reach the same conclusion
// from the same signals; two separately written judgements eventually disagree at
// some boundary, and the whole point of a rehearsal is that it does not.
type TradingStrategyBacktestDomain struct {
	backtest      BacktestDomain
	sources       []resolvedSignalSource
	buyCondition  TradingStrategyConditionDomain
	sellCondition TradingStrategyConditionDomain
}

// resolvedSignalSource is one source as this replay will run it: what to call it in
// the conditions, the script, and its knobs already settled.
type resolvedSignalSource struct {
	label      string
	script     string
	parameters StrategyScriptParametersDomain
}

// NewTradingStrategyBacktestDomain validates the request against every replay rule —
// the shared coarseness first, because it is the one that decides which candles the
// rest of the rules are even about.
func NewTradingStrategyBacktestDomain(
	requestDto dto.TradingStrategyBacktestRequestDto, maxCandleCount int, now time.Time,
) (TradingStrategyBacktestDomain, error) {
	// The same question the save gate asks, in the same words — this is the second
	// of the two, kept because a trading strategy saved before that gate existed can
	// still be mixed.
	intervals := make([]string, 0, len(requestDto.SignalSources))
	for _, signalSource := range requestDto.SignalSources {
		intervals = append(intervals, signalSource.AggregationInterval)
	}

	sharedInterval, intervalError := NewSharedAggregationIntervalDomain(intervals).Shared()
	if intervalError != nil {
		return TradingStrategyBacktestDomain{}, BacktestValidationFailure(
			BacktestSignalSourcesField, intervalError.Error())
	}

	// The coarseness is the trading strategy's answer, not the caller's, which is the
	// only thing this has to supply — every other condition of the replay travels
	// with the request itself.
	backtest, backtestError := NewBacktestDomain(
		requestDto.ToBacktestRequestDto(sharedInterval), maxCandleCount, now)
	if backtestError != nil {
		return TradingStrategyBacktestDomain{}, backtestError
	}

	sources := make([]resolvedSignalSource, 0, len(requestDto.SignalSources))
	labels := make([]string, 0, len(requestDto.SignalSources))
	for _, signalSource := range requestDto.SignalSources {
		declaredParameters, parametersError := NewStrategyScriptParametersDomain(
			signalSource.Parameters)
		if parametersError != nil {
			return TradingStrategyBacktestDomain{}, fmt.Errorf(
				"%w: %w", ErrBacktestValidation, parametersError)
		}

		parameters, applyError := declaredParameters.Applying(signalSource.ParameterValues)
		if applyError != nil {
			return TradingStrategyBacktestDomain{}, fmt.Errorf(
				"%w: %w", ErrBacktestValidation, applyError)
		}

		label := strings.TrimSpace(signalSource.Label)
		sources = append(sources, resolvedSignalSource{
			label:      label,
			script:     signalSource.Script,
			parameters: parameters,
		})
		labels = append(labels, label)
	}

	buyCondition, buyError := NewTradingStrategyConditionDomain(requestDto.BuyCondition, labels)
	if buyError != nil {
		return TradingStrategyBacktestDomain{}, fmt.Errorf("%w（買入條件）", buyError)
	}

	sellCondition, sellError := NewTradingStrategyConditionDomain(requestDto.SellCondition, labels)
	if sellError != nil {
		return TradingStrategyBacktestDomain{}, fmt.Errorf("%w（賣出條件）", sellError)
	}

	return TradingStrategyBacktestDomain{
		backtest:      backtest,
		sources:       sources,
		buyCondition:  buyCondition,
		sellCondition: sellCondition,
	}, nil
}

// Symbol, KCandleQuery, SourceCandleLimit and SelectInputCandles are the replay's,
// and they are the replay's untouched: which candles a stretch is made of does not
// change because several scripts read them instead of one.
func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) Symbol() string {
	return tradingStrategyBacktestDomain.backtest.Symbol()
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) KCandleQuery() KCandleQueryDomain {
	return tradingStrategyBacktestDomain.backtest.KCandleQuery()
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SourceCandleLimit() int {
	return tradingStrategyBacktestDomain.backtest.SourceCandleLimit()
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SelectInputCandles(
	kCandles []entities.KCandle,
) ([]vo.KCandleVo, error) {
	return tradingStrategyBacktestDomain.backtest.SelectInputCandles(kCandles)
}

// ResultType is the kind every source's script is run under. A replay reads opinions,
// so all of them are run as signals — a source that produces numbers has nothing to
// say to a condition.
func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) ResultType() IndicatorResultTypeDomain {
	return tradingStrategyBacktestDomain.backtest.ResultType()
}

// SourceCount is how many scripts this replay has to run.
func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SourceCount() int {
	return len(tradingStrategyBacktestDomain.sources)
}

// SourceScript and SourceParameters are what to run for the nth source. They are two
// reads rather than one struct because the script proxy takes them as two arguments,
// and handing out a shape it would immediately unpack buys nothing.
func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SourceScript(index int) string {
	return tradingStrategyBacktestDomain.sources[index].script
}

func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) SourceParameters(
	index int,
) StrategyScriptParametersDomain {
	return tradingStrategyBacktestDomain.sources[index].parameters
}

// ReplayOver turns what every source said about every candle into one opinion per
// candle, then walks the account over them.
//
// signalsBySource is one slice per source, in the order the sources were given, each
// as long as the candles. Anything shorter would mean a source that stopped having
// opinions half way, and a replay that quietly treats the rest as "hold" is a replay
// that reports a strategy nobody ran.
func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) ReplayOver(
	inputKCandles []vo.KCandleVo, signalsBySource [][]SignalDomain,
) dto.BacktestResultDto {
	verdicts, conflictedCandleCount := tradingStrategyBacktestDomain.combineSignals(
		len(inputKCandles), signalsBySource)

	backtestResultDto := tradingStrategyBacktestDomain.backtest.ReplayOver(inputKCandles, verdicts)
	backtestResultDto.Summary.ConflictedCandleCount = conflictedCandleCount

	return backtestResultDto
}

// combineSignals reads each candle's sources through the two condition trees.
//
// Both conditions holding is a conflict, and a conflict does nothing — the same
// answer a live round gives, for the same reason: picking a side would hand somebody
// an opinion the system invented, and they would act on it without ever tracing it
// back. They are counted so that it is visible rather than merely true.
func (tradingStrategyBacktestDomain TradingStrategyBacktestDomain) combineSignals(
	candleCount int, signalsBySource [][]SignalDomain,
) ([]SignalDomain, int) {
	verdicts := make([]SignalDomain, 0, candleCount)
	conflictedCandleCount := 0

	for candleIndex := range candleCount {
		signalsByLabel := make(map[string]vo.SignalVo, len(tradingStrategyBacktestDomain.sources))
		for sourceIndex, source := range tradingStrategyBacktestDomain.sources {
			signalsByLabel[source.label] = signalsBySource[sourceIndex][candleIndex].Value()
		}

		// The last sent signal is empty here on purpose. It is what decides whether a
		// live round *speaks*, and a replay never speaks — it acts on every candle.
		verdictDomain := NewStrategyBotVerdictDomain(
			tradingStrategyBacktestDomain.buyCondition.Holds(signalsByLabel),
			tradingStrategyBacktestDomain.sellCondition.Holds(signalsByLabel),
			"")

		if verdictDomain.IsConflicting() {
			conflictedCandleCount++
		}

		// A conflict and a quiet candle both come out as hold, which is what doing
		// nothing looks like to the account.
		signal, hasSignal := verdictDomain.Signal()
		if !hasSignal {
			signal = vo.SignalHold
		}

		verdicts = append(verdicts, NewSignalDomainOf(signal))
	}

	return verdicts, conflictedCandleCount
}
