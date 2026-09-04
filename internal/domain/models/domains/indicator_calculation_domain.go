package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// spareBucketCount is the one bucket beyond what was asked for that every read
// reaches back over.
//
// A read stops at a fixed number of stored candles, and that number does not land on
// a bucket edge — so the earliest bucket it reached may hold only its later half, and
// merging that would understate its opening price and its volumes. The spare is what
// makes that bucket harmless: a read that stopped at its limit has necessarily
// reached into one more bucket than was asked for, and since the candles handed over
// are always the latest ones, the earliest — the only one a limit can cut in half —
// is never among them. Nothing has to notice the truncation or discard anything.
const spareBucketCount = 1

// IndicatorCalculationDomain holds one calculation request and guarantees its own
// invariants. It also owns every rule that decides which K candles the script sees:
// how coarse they are, how many, up to when, and that a bucket still running is
// never one of them.
//
// A strategy holds none of this. How coarse and how many describe one run rather
// than the algorithm, so they arrive here — which is what lets one algorithm be run
// at any coarseness over any stretch of market.
type IndicatorCalculationDomain struct {
	symbol      string
	candleCount int
	parameters  StrategyParametersDomain
	resultType  IndicatorResultTypeDomain
	interval    AggregationIntervalDomain
	// endTime is the moment this calculation reaches up to, already settled: never
	// zero and never in the future, so nothing downstream has to ask again.
	endTime time.Time
}

// NewIndicatorCalculationDomain validates the request against every request rule.
//
// The current moment is passed in rather than read here, so that what a calculation
// answers stays decided by its arguments — a rule about "now" that reads the wall
// clock cannot be checked.
func NewIndicatorCalculationDomain(
	requestDto dto.IndicatorCalculationRequestDto, maxCandleCount int, now time.Time,
) (IndicatorCalculationDomain, error) {
	tradingSymbol, symbolError := NewTradingSymbolDomain(requestDto.Symbol)
	if symbolError != nil {
		return IndicatorCalculationDomain{},
			fmt.Errorf("%w: %w", ErrIndicatorCalculationValidation, symbolError)
	}

	if requestDto.CandleCount <= 0 {
		return IndicatorCalculationDomain{},
			fmt.Errorf("%w: 計算根數必須大於零", ErrIndicatorCalculationValidation)
	}

	declaredParameters, parametersError := NewStrategyParametersDomain(requestDto.Parameters)
	if parametersError != nil {
		return IndicatorCalculationDomain{}, fmt.Errorf(
			"%w: %w", ErrIndicatorCalculationValidation, parametersError)
	}

	parameters, applyError := declaredParameters.Applying(requestDto.ParameterValues)
	if applyError != nil {
		return IndicatorCalculationDomain{}, fmt.Errorf(
			"%w: %w", ErrIndicatorCalculationValidation, applyError)
	}

	// The caller asks for however many candles it wants a value for; the algorithm
	// needs that many plus whatever its hungriest knob reaches back over, less the
	// one they share.
	//
	// The "less one" only applies once there is something to reach back over: a
	// look-back of twenty produces its first value on the twentieth candle, so it
	// costs nineteen extra. An algorithm that declares no look-back at all costs
	// nothing extra — not one candle less, which is what subtracting unconditionally
	// would quietly do.
	inputCandleCount := requestDto.CandleCount + max(0, parameters.MaximumLookbackCount()-1)

	// The ceiling counts aggregated candles, not the stored ones behind them: asking
	// for a day at one-hour buckets asks for 24 candles however many five-minute
	// candles were read to build them. It is judged against what will actually be
	// fed to the algorithm, not against what was asked for — a modest span with a
	// long look-back can exceed it, and refusing only on the asked-for number would
	// let that through and fail further in.
	if inputCandleCount > maxCandleCount {
		return IndicatorCalculationDomain{}, CandleCountExceeded(inputCandleCount, maxCandleCount)
	}

	interval, intervalError := NewAggregationIntervalDomain(requestDto.AggregationInterval)
	if intervalError != nil {
		return IndicatorCalculationDomain{}, fmt.Errorf(
			"%w: %w", ErrIndicatorCalculationValidation, intervalError)
	}

	resultType, resultTypeError := NewIndicatorResultTypeDomain(requestDto.ResultType)
	if resultTypeError != nil {
		return IndicatorCalculationDomain{}, fmt.Errorf(
			"%w: %w", ErrIndicatorCalculationValidation, resultTypeError)
	}

	return IndicatorCalculationDomain{
		symbol:      tradingSymbol.Value(),
		candleCount: inputCandleCount,
		parameters:  parameters,
		resultType:  resultType,
		interval:    interval,
		endTime:     effectiveEndTime(requestDto.EndTime, now),
	}, nil
}

// effectiveEndTime settles what "up to when" means. Naming no moment means now, and
// naming one that has not arrived also means now — the market cannot be read past
// the present, and refusing would break the ordinary case of a chart scrolled a
// little past its right edge.
func effectiveEndTime(declaredEndTime time.Time, now time.Time) time.Time {
	if declaredEndTime.IsZero() || declaredEndTime.After(now) {
		return now
	}

	return declaredEndTime
}

func (indicatorCalculationDomain IndicatorCalculationDomain) Symbol() string {
	return indicatorCalculationDomain.symbol
}

// ResultType is the indicator value kind this request declared, already read and
// accepted. Everything downstream takes the kind from here rather than from the raw
// declaration, so a declaration is only ever interpreted once.
func (indicatorCalculationDomain IndicatorCalculationDomain) ResultType() IndicatorResultTypeDomain {
	return indicatorCalculationDomain.resultType
}

// Interval is how coarse this calculation reads the market, already read and
// accepted, for the same reason.
func (indicatorCalculationDomain IndicatorCalculationDomain) Interval() AggregationIntervalDomain {
	return indicatorCalculationDomain.interval
}

// ReadCutoff is the moment to stop reading at: only K candles that opened strictly
// before it may be read.
//
// It is the start of the bucket the end time falls into, which is always the bucket
// still running — so the bucket that has not finished is never read at all, rather
// than read and then thrown away. At five-minute buckets this comes out as the old
// rule of leaving out the newest candle; at one hour it leaves out the seven candles
// of an hour that is 35 minutes old, which the old rule never could.
//
// It holds equally for an end time long past. A bucket cut off half way through is
// half-formed whenever it happened, and a value computed from it would change if the
// same question were asked with a slightly later end time.
func (indicatorCalculationDomain IndicatorCalculationDomain) ReadCutoff() time.Time {
	return indicatorCalculationDomain.interval.BucketStart(indicatorCalculationDomain.endTime)
}

// SourceCandleLimit is the most stored K candles worth reading: as many as the
// buckets asked for can hold, plus the one spare bucket. It can never cut the answer
// short, and it stops an over-wide read before it starts.
//
// Reading a number of candles rather than a stretch of time is also what makes gaps
// free: where a stretch of market is missing, the same number of candles simply
// reaches further back, and the empty buckets in between are skipped without a rule
// for skipping them.
func (indicatorCalculationDomain IndicatorCalculationDomain) SourceCandleLimit() int {
	return indicatorCalculationDomain.interval.SourceCandleCount(
		indicatorCalculationDomain.candleCount + spareBucketCount)
}

// SelectInputCandles takes the K candles as read — newest first, none of them from a
// bucket still running — and hands back what the script sees: one candle per
// finished bucket, as many as were asked for, oldest first. Too few is refused,
// naming how many there actually were.
func (indicatorCalculationDomain IndicatorCalculationDomain) SelectInputCandles(
	newestFirstKCandles []entities.KCandle,
) ([]vo.KCandleVo, error) {
	buckets := NewKCandleSeriesDomain(
		indicatorCalculationDomain.symbol,
		indicatorCalculationDomain.interval,
		newestFirstKCandles,
	).Buckets()

	if len(buckets) < indicatorCalculationDomain.candleCount {
		return nil, fmt.Errorf(
			"%w: K 線不足，走完的刻度區間目前湊得出 %d 根，但要求 %d 根",
			ErrIndicatorCalculationValidation, len(buckets), indicatorCalculationDomain.candleCount)
	}

	// The latest ones, which is also what keeps a bucket the read cut in half out of
	// the answer: see spareBucketCount.
	latestBuckets := buckets[len(buckets)-indicatorCalculationDomain.candleCount:]

	oldestFirstKCandleVos := make([]vo.KCandleVo, 0, indicatorCalculationDomain.candleCount)
	for _, bucket := range latestBuckets {
		oldestFirstKCandleVos = append(oldestFirstKCandleVos, bucket.ToVo())
	}

	return oldestFirstKCandleVos, nil
}

// Parameters are this run's knobs, already settled: every declared one carries the
// value it will be read with, whether that came from the run or from the declaration.
func (indicatorCalculationDomain IndicatorCalculationDomain) Parameters() StrategyParametersDomain {
	return indicatorCalculationDomain.parameters
}
