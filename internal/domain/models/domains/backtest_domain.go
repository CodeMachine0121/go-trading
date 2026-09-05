package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// minimumBacktestKCandleCount is the fewest candles a replay can say anything with.
//
// One candle has no "before" and no "after": whatever the strategy decides on it, the
// account never reaches a second price to be judged at. Two is the first number where
// a decision has a consequence.
const minimumBacktestKCandleCount = 2

// spareBacktestBucketCount is the one bucket beyond the stretch asked for that the
// read reaches over, so a limit can never cut the answer short.
const spareBacktestBucketCount = 1

// BacktestDomain is one replay's conditions, and every rule about what may be
// replayed: which market, how coarse, which stretch, which algorithm with which knobs,
// how much capital, and how much each opening stakes.
//
// It stops at "what may be replayed". What happens once the candles are in hand
// belongs to BacktestSimulationDomain, which this hands over to — the two change for
// different reasons and would otherwise be one file edited by two unrelated needs.
type BacktestDomain struct {
	symbol         string
	interval       AggregationIntervalDomain
	parameters     StrategyParametersDomain
	initialCapital decimal.Decimal
	positionSizing PositionSizingDomain
	startTime      time.Time
	// readCutoff is the moment to stop reading at, already settled: only candles from
	// buckets that opened strictly before it are replayed.
	readCutoff time.Time
}

// NewBacktestDomain validates the request against every replay rule.
//
// The current moment is passed in rather than read here, so that what a replay answers
// stays decided by its arguments — a rule about "now" that reads the wall clock cannot
// be checked.
func NewBacktestDomain(
	requestDto dto.BacktestRequestDto, maxCandleCount int, now time.Time,
) (BacktestDomain, error) {
	tradingSymbol, symbolError := NewTradingSymbolDomain(requestDto.Symbol)
	if symbolError != nil {
		return BacktestDomain{}, fmt.Errorf("%w: %w", ErrBacktestValidation, symbolError)
	}

	interval, intervalError := NewAggregationIntervalDomain(requestDto.AggregationInterval)
	if intervalError != nil {
		return BacktestDomain{}, fmt.Errorf("%w: %w", ErrBacktestValidation, intervalError)
	}

	if !requestDto.InitialCapital.IsPositive() {
		return BacktestDomain{}, fmt.Errorf(
			"%w: 初始資金必須大於零", ErrBacktestValidation)
	}

	positionSizing, positionSizingError := NewPositionSizingDomain(
		requestDto.PositionSizingMode, requestDto.PositionSizingValue)
	if positionSizingError != nil {
		return BacktestDomain{}, positionSizingError
	}

	declaredParameters, parametersError := NewStrategyParametersDomain(requestDto.Parameters)
	if parametersError != nil {
		return BacktestDomain{}, fmt.Errorf("%w: %w", ErrBacktestValidation, parametersError)
	}

	parameters, applyError := declaredParameters.Applying(requestDto.ParameterValues)
	if applyError != nil {
		return BacktestDomain{}, fmt.Errorf("%w: %w", ErrBacktestValidation, applyError)
	}

	startTime := requestDto.StartTime.UTC()
	// The bucket the stretch ends in is the one still running as far as this replay is
	// concerned: it is only half inside the stretch asked for, and a value computed
	// from half a bucket changes as soon as the same question is asked a moment later.
	// This is the same rule an indicator calculation reads by, deliberately.
	readCutoff := interval.BucketStart(effectiveEndTime(requestDto.EndTime, now))

	// Nothing finished inside the stretch — because it ends before it starts, or
	// because it is shorter than one bucket. Both are the same thing to whoever asked,
	// so both get the sentence about not having enough candles rather than one of them
	// getting a rule of its own.
	if !readCutoff.After(startTime) {
		return BacktestDomain{}, notEnoughKCandlesForBacktest(0)
	}

	bucketCount := interval.BucketCount(startTime, readCutoff)
	if bucketCount > maxCandleCount {
		return BacktestDomain{}, fmt.Errorf(
			"%w: 這一段以這個彙總刻度要用到 %d 根，超過單次可用的最大根數（最多 %d 根）",
			ErrBacktestValidation, bucketCount, maxCandleCount)
	}

	return BacktestDomain{
		symbol:         tradingSymbol.Value(),
		interval:       interval,
		parameters:     parameters,
		initialCapital: requestDto.InitialCapital,
		positionSizing: positionSizing,
		startTime:      startTime,
		readCutoff:     readCutoff,
	}, nil
}

func (backtestDomain BacktestDomain) Symbol() string {
	return backtestDomain.symbol
}

func (backtestDomain BacktestDomain) Interval() AggregationIntervalDomain {
	return backtestDomain.interval
}

// Parameters are this run's knobs, already settled: every declared one carries the
// value it will be read with, whether that came from the run or from the declaration.
func (backtestDomain BacktestDomain) Parameters() StrategyParametersDomain {
	return backtestDomain.parameters
}

// ResultType is the kind of value a replayed script produces. It is always one number
// per indicator, because a signal is one number — so unlike an indicator calculation
// there is nothing here for a caller to declare, and nothing to get wrong.
func (backtestDomain BacktestDomain) ResultType() IndicatorResultTypeDomain {
	return IndicatorResultTypeDomain{value: vo.IndicatorResultTypeFloat}
}

// KCandleQuery is the stretch of storage to read: from the start asked for up to the
// cut-off. It is built here rather than by the caller because the cut-off is this
// model's answer, and a caller free to name its own end could quietly read a bucket
// still running.
//
// The cut-off is included in the query and excluded from the replay, which costs one
// harmless extra candle and keeps the boundary written as the rule reads: replay only
// what opened strictly before the cut-off.
func (backtestDomain BacktestDomain) KCandleQuery() KCandleQueryDomain {
	return KCandleQueryDomain{
		symbol:    backtestDomain.symbol,
		startTime: backtestDomain.startTime,
		endTime:   backtestDomain.readCutoff,
	}
}

// SourceCandleLimit is the most stored K candles worth reading for this stretch: as
// many as its buckets can hold, plus one spare bucket. It can never cut the answer
// short, and it stops an over-wide read before it starts.
func (backtestDomain BacktestDomain) SourceCandleLimit() int {
	return backtestDomain.interval.SourceCandleCount(
		backtestDomain.interval.BucketCount(backtestDomain.startTime, backtestDomain.readCutoff) +
			spareBacktestBucketCount)
}

// SelectInputCandles takes the K candles as read — earliest first — and hands back
// what the script is replayed over: one candle per finished bucket, oldest first.
// Fewer than two is refused, naming how many there actually were.
func (backtestDomain BacktestDomain) SelectInputCandles(
	kCandles []entities.KCandle,
) ([]vo.KCandleVo, error) {
	buckets := NewKCandleSeriesDomain(
		backtestDomain.symbol, backtestDomain.interval, kCandles).Buckets()

	inputKCandleVos := make([]vo.KCandleVo, 0, len(buckets))
	for _, bucket := range buckets {
		if !bucket.OpenTime().Before(backtestDomain.readCutoff) {
			continue
		}

		inputKCandleVos = append(inputKCandleVos, bucket.ToVo())
	}

	if len(inputKCandleVos) < minimumBacktestKCandleCount {
		return nil, notEnoughKCandlesForBacktest(len(inputKCandleVos))
	}

	return inputKCandleVos, nil
}

// Simulation hands the conditions over to the walk, so that the capital and the sizing
// mode travel with them instead of being carried separately by a caller that would
// then be free to pair the wrong ones.
func (backtestDomain BacktestDomain) Simulation(
	inputKCandles []vo.KCandleVo, perCandleIndicatorValues []map[string]vo.IndicatorValueVo,
) BacktestSimulationDomain {
	return NewBacktestSimulationDomain(
		backtestDomain.initialCapital,
		backtestDomain.positionSizing,
		inputKCandles,
		perCandleIndicatorValues)
}
