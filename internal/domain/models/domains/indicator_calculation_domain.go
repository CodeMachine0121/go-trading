package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// spareBucketCount is the one bucket beyond what was asked for that every read
// reaches over, whether it reaches backwards from a moment or across a stretch.
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
	symbol string
	// candleCount is how many finished buckets it would take to fill every position
	// the caller is looking at. It is what the read is sized for and what a full
	// answer holds; a short stretch answers with fewer, never with an error.
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
	// for a day at one-hour buckets asks for 24 candles however many one-minute
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
// than read and then thrown away. At one-minute buckets this comes out as the plain
// rule of leaving out the newest candle; at one hour it leaves out the thirty-five
// candles of an hour that is 35 minutes old, which the plain rule never could.
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
// bucket still running — and hands back what the script sees: one candle per finished
// bucket, oldest first, as many as were asked for or as many as there are, whichever
// is fewer.
//
// Coming up short is not a refusal. The count asked for is worked out from how wide a
// stretch the caller is looking at, and a chart looking further back than the stored
// history reaches has not asked anything wrong — it has asked about a stretch that is
// only partly there. Answering over what is there hands back a shorter line, which is
// the honest answer; refusing hands back nothing and leaves the reader guessing which
// coarseness would have worked.
//
// The one shortfall that cannot be answered is a stretch below what the declared
// look-back reaches over: not one value can come out, so there is nothing to hand
// over. That floor is worked out here, next to the comparison that uses it.
//
// Taking every bucket when short is safe against the truncation spareBucketCount
// guards, and it is worth saying why, because this is the first read that keeps the
// earliest bucket. The read limit holds as many stored candles as the count asked for
// plus one bucket, and a trading symbol has at most one candle per slot — so a read
// that reached its limit has necessarily merged at least that many buckets. Coming
// out with fewer buckets than were asked for therefore proves the read never reached
// its limit: storage was exhausted, and no cut by the limit is in the answer.
//
// **What that does not prove is that the earliest bucket is full**, and at a
// coarseness that merges several candles it often is not. Ingestion backfills from a
// plain wall-clock moment, which lands nowhere near a bucket edge, so the oldest
// stored candle sits somewhere inside its bucket: at one day, the earliest bucket of
// a symbol backfilled from 14:03 merges ten hours of trading and is presented as a
// day, with that afternoon's opening price and roughly half a day's volumes. A short
// answer's first value can be computed from it, and usedCandleCount counts it as a
// bucket like any other.
//
// It is left in, and that is a decision rather than an oversight. A single candle at
// 07:35 is either an hour the market barely traded or an hour storage only caught the
// end of, and nothing here can tell those apart — the same ambiguity that stops this
// calculation from demanding a full bucket at the live edge, where the choice was
// also to answer and name the limit. Dropping it would cost every coarse read one
// value (N values would want N+1 buckets of history) and would refuse thin symbols a
// value they had earned. Naming it is the cheaper honesty; the caller that needs a
// settled first value asks over a stretch it knows is fully stored.
func (indicatorCalculationDomain IndicatorCalculationDomain) SelectInputCandles(
	newestFirstKCandles []entities.KCandle,
) ([]vo.KCandleVo, error) {
	buckets := NewKCandleSeriesDomain(
		indicatorCalculationDomain.symbol,
		indicatorCalculationDomain.interval,
		newestFirstKCandles,
	).Buckets()

	// The fewest finished buckets this calculation can say anything at all from: the
	// look-back its hungriest knob declares, because an algorithm reaching back over
	// twenty candles produces its first value on the twentieth. One when nothing
	// reaches back, never zero — a calculation over no market at all has no answer,
	// and the alternative is handing an empty batch to a script to fail inside.
	//
	// It reads only what the strategy *declares*. What an algorithm actually reaches
	// for stays the algorithm's own business to guard, which is why a script
	// hard-coding a period it never declared comes back as a script failure rather
	// than as a shortfall: the system does not guess how many an algorithm needs.
	minimumCandleCount := max(1, indicatorCalculationDomain.parameters.MaximumLookbackCount())
	if len(buckets) < minimumCandleCount {
		return nil, CandleCoverageTooThin(len(buckets), minimumCandleCount)
	}

	// The latest ones, which is also what keeps a bucket the read cut in half out of
	// the answer whenever there are more than were asked for: see spareBucketCount.
	usedCandleCount := min(indicatorCalculationDomain.candleCount, len(buckets))
	latestBuckets := buckets[len(buckets)-usedCandleCount:]

	oldestFirstKCandleVos := make([]vo.KCandleVo, 0, usedCandleCount)
	for _, bucket := range latestBuckets {
		oldestFirstKCandleVos = append(oldestFirstKCandleVos, bucket.ToVo())
	}

	return oldestFirstKCandleVos, nil
}

// CandleCount is how many finished buckets this calculation would need to have a
// value for every position the caller is looking at. Answered alongside how many were
// actually used, it is what lets a caller say a stretch came out short without
// working the number out a second time from the span and the look-back — and the day
// the two derivations disagree, the sentence would be wrong with nothing reported.
func (indicatorCalculationDomain IndicatorCalculationDomain) CandleCount() int {
	return indicatorCalculationDomain.candleCount
}

// Parameters are this run's knobs, already settled: every declared one carries the
// value it will be read with, whether that came from the run or from the declaration.
func (indicatorCalculationDomain IndicatorCalculationDomain) Parameters() StrategyParametersDomain {
	return indicatorCalculationDomain.parameters
}
