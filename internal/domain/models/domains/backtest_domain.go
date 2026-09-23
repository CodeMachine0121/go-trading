package domains

import (
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// minimumBacktestKCandleCount is the fewest candles a replay can say anything with.
//
// One candle has no "before" and no "after": whatever the strategy script decides on it, the
// account never reaches a second price to be judged at. Two is the first number where
// a decision has a consequence.
const minimumBacktestKCandleCount = 2

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
	parameters     StrategyScriptParametersDomain
	initialCapital decimal.Decimal
	positionTerms  BacktestPositionTermsDomain
	fillTiming     BacktestFillTimingDomain
	segments       BacktestSegmentsDomain
	startTime      time.Time
	// readCutoff is the moment to stop reading at, already settled: only candles from
	// buckets that opened strictly before it are replayed.
	readCutoff time.Time
}

// effectiveEndTime settles what "up to when" means for a replay. Naming no moment
// means now, and naming one that has not arrived also means now — the market cannot
// be read past the present, and refusing would break the ordinary case of a stretch
// asked for a little past the right edge.
//
// It lives here because a replay is the only thing left that reads a raw declaration:
// an indicator calculation now takes an observation window, which settles the same
// rule at construction. The day a replay takes one too, this goes with it.
func effectiveEndTime(declaredEndTime time.Time, now time.Time) time.Time {
	if declaredEndTime.IsZero() || declaredEndTime.After(now) {
		return now
	}

	return declaredEndTime
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

	// Asked before anything else is built, because what it refuses is a request for a
	// different kind of replay altogether — there is no point checking how much of the
	// cash it would stake.
	//
	// Which box to point at is this replay's business rather than the refusal's: the
	// sentence is shared with three other callers, and each of them calls its boxes
	// something different.
	if _, spotOnlyRefusal := NewSpotOnlyReplayDomain(
		requestDto.TradingMode, requestDto.Leverage,
		requestDto.MaintenanceMarginRate); spotOnlyRefusal != nil {
		refusedField := BacktestLeverageField
		if errors.Is(spotOnlyRefusal, ErrSpotOnlyTradingMode) {
			refusedField = BacktestTradingModeField
		}

		return BacktestDomain{}, BacktestValidationFailure(
			refusedField, spotOnlyRefusal.Error())
	}

	interval, intervalError := NewAggregationIntervalDomain(requestDto.AggregationInterval)
	if intervalError != nil {
		return BacktestDomain{}, fmt.Errorf("%w: %w", ErrBacktestValidation, intervalError)
	}

	if !requestDto.InitialCapital.IsPositive() {
		return BacktestDomain{}, BacktestValidationFailure(
			BacktestInitialCapitalField, "初始資金必須大於零")
	}

	positionSizing, positionSizingError := NewPositionSizingDomain(
		requestDto.PositionSizingMode, requestDto.PositionSizingValue)
	if positionSizingError != nil {
		// The sentence comes from the model; which input this replay calls the thing
		// at fault is its own business. Only the figure is an input a caller can go
		// and change — a mode nobody offers is answered by listing the three — so
		// only that one names a field. A bot being saved wraps the same sentences in
		// its own sentinel instead.
		if PositionSizingFailureAboutFigure(positionSizingError) {
			return BacktestDomain{}, BacktestValidationFailure(
				BacktestPositionSizingValueField, positionSizingError.Error())
		}

		return BacktestDomain{}, fmt.Errorf(
			"%w: %s", ErrBacktestValidation, positionSizingError)
	}

	exitLevels, exitLevelsError := NewBacktestExitLevelsDomain(
		requestDto.StopLossPercentage, requestDto.TakeProfitPercentage)
	if exitLevelsError != nil {
		// The sentence comes from the distances themselves — the very sentence a bot
		// being saved gets for the same figure — and naming which input it is about
		// is this replay's business. One name covers both distances; the sentence
		// says which.
		return BacktestDomain{}, BacktestValidationFailure(
			BacktestExitLevelsField, exitLevelsError.Error())
	}

	transactionCosts, transactionCostsError := NewBacktestTransactionCostsDomain(
		requestDto.EntryCostPercentage, requestDto.ExitCostPercentage)
	if transactionCostsError != nil {
		// One name covers both rates, and the sentence says which. They are filled in
		// as one group on every screen that offers them — the same judgement the exit
		// distances make, and the same one the time range makes with two moments and
		// a coarseness.
		return BacktestDomain{}, BacktestValidationFailure(
			BacktestTransactionCostsField, transactionCostsError.Error())
	}

	// A percentage bigger than the share the costs leave affordable can never open
	// anything — not on this candle, on any candle. Left to run, it hands back a
	// report card of a strategy that never traded, and every word on that screen
	// points at the algorithm instead of at the two numbers that caused it.
	//
	// It points at the percentage rather than at the rates because the rates are a
	// fact about somebody's broker and the percentage is the knob.
	positionTerms := NewBacktestPositionTermsDomain(
		positionSizing, exitLevels, transactionCosts)

	if positionTerms.NeverOpensAnything() {
		return BacktestDomain{}, BacktestValidationFailure(
			BacktestPositionSizingValueField,
			"這個百分比連同它的進場成本付不起，每一次開倉都會被跳過，"+
				"這次重演一筆交易都不會有。要押滿請改用全押——它會自己留出手續費")
	}

	declaredParameters, parametersError := NewStrategyScriptParametersDomain(requestDto.Parameters)
	if parametersError != nil {
		return BacktestDomain{}, fmt.Errorf("%w: %w", ErrBacktestValidation, parametersError)
	}

	parameters, applyError := declaredParameters.Applying(requestDto.ParameterValues)
	if applyError != nil {
		return BacktestDomain{}, fmt.Errorf("%w: %w", ErrBacktestValidation, applyError)
	}

	fillTiming, fillTimingError := NewBacktestFillTimingDomain(requestDto.FillTiming)
	if fillTimingError != nil {
		return BacktestDomain{}, BacktestValidationFailure(BacktestFillTimingField, fillTimingError.Error())
	}

	startTime := requestDto.StartTime.UTC()
	endTime := effectiveEndTime(requestDto.EndTime, now)
	// The bucket the stretch ends in is the one still running as far as this replay is
	// concerned: it is only half inside the stretch asked for, and a value computed
	// from half a bucket changes as soon as the same question is asked a moment later.
	// This is the same rule an indicator calculation reads by, deliberately.
	readCutoff := interval.BucketStart(endTime)

	segments, segmentsError := NewBacktestSegmentsDomain(requestDto.ValidationStartTime, startTime, endTime)
	if segmentsError != nil {
		return BacktestDomain{}, segmentsError
	}

	// Nothing finished inside the stretch — because it ends before it starts, or
	// because it is shorter than one bucket. Both are the same thing to whoever asked,
	// so both get the sentence about not having enough candles rather than one of them
	// getting a rule of its own.
	if !readCutoff.After(startTime) {
		return BacktestDomain{}, notEnoughKCandlesForBacktest(0)
	}

	bucketCount := interval.BucketCount(startTime, readCutoff)
	if bucketCount > maxCandleCount {
		return BacktestDomain{}, BacktestValidationFailure(
			BacktestTimeRangeField,
			fmt.Sprintf("這一段以這個彙總刻度要用到 %d 根，超過單次可用的最大根數（最多 %d 根）",
				bucketCount, maxCandleCount))
	}

	return BacktestDomain{
		symbol:         tradingSymbol.Value(),
		interval:       interval,
		parameters:     parameters,
		initialCapital: requestDto.InitialCapital,
		positionTerms:  positionTerms,
		fillTiming:     fillTiming,
		segments:       segments,
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
func (backtestDomain BacktestDomain) Parameters() StrategyScriptParametersDomain {
	return backtestDomain.parameters
}

// ResultType is the kind of value a replayed script produces. It is always the
// signal kind: a replay reads an opinion off every candle, so a script that produces
// anything else — a number, an answer — is one a replay cannot act on and is refused.
// There is nothing here for a caller to declare, and nothing to get wrong.
//
// Replaying a whole trading strategy asks nothing extra of this: its sources may
// only name strategy scripts declared as signals, settled when it was saved, so
// what arrives here already agrees with what is returned here.
func (backtestDomain BacktestDomain) ResultType() IndicatorResultTypeDomain {
	return IndicatorResultTypeDomain{value: vo.IndicatorResultTypeSignal}
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
			spareBucketCount)
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

	// A split replay needs a bar on either side of the validation start; refused here,
	// before a single script is run, rather than after.
	if _, splitError := backtestDomain.splitIndexOf(inputKCandleVos); splitError != nil {
		return nil, splitError
	}

	return inputKCandleVos, nil
}

// splitIndexOf is where these candles divide into the in-sample and the validation
// part — asked once when they are selected, to refuse early, and once when they are
// replayed. An unsplit replay divides nowhere and answers the whole length.
func (backtestDomain BacktestDomain) splitIndexOf(inputKCandles []vo.KCandleVo) (int, error) {
	if !backtestDomain.segments.IsSplit() {
		return len(inputKCandles), nil
	}

	openTimes := make([]time.Time, 0, len(inputKCandles))
	for _, inputKCandle := range inputKCandles {
		openTimes = append(openTimes, time.Unix(inputKCandle.OpenTimeUnixSeconds, 0).UTC())
	}

	return backtestDomain.segments.SplitIndex(openTimes)
}

// ReplayOver walks the candles and hands back the whole result, conditions and all —
// and for a split replay walks each part again on its own, from the initial capital and
// flat. The opinions are the same ones throughout: a script standing on the first
// validation candle has seen everything before it, exactly as it would have then.
//
// It is one call rather than "build a walk, run it, then say which market and which
// coarseness it was" because those answers are this model's, not the walk's.
//
// conflictedFlags says, candle by candle, whether a trading strategy's two trees both
// held; a strategy script replay has no such thing and passes nil.
func (backtestDomain BacktestDomain) ReplayOver(
	inputKCandles []vo.KCandleVo, signals []SignalDomain, conflictedFlags []bool,
) dto.BacktestResultDto {
	if conflictedFlags == nil {
		conflictedFlags = make([]bool, len(inputKCandles))
	}

	replayed := func(
		partKCandles []vo.KCandleVo, partSignals []SignalDomain, partConflictedFlags []bool,
	) dto.BacktestResultDto {
		backtestResultDto := NewBacktestSimulationDomain(
			backtestDomain.initialCapital,
			backtestDomain.positionTerms,
			backtestDomain.fillTiming,
			partKCandles,
			partSignals).ToDto()

		backtestResultDto.Symbol = backtestDomain.symbol
		backtestResultDto.Interval = string(backtestDomain.interval.Value())
		backtestResultDto.StartTime = backtestResultDto.EquityCurve[0].OpenTime
		backtestResultDto.EndTime =
			backtestResultDto.EquityCurve[len(backtestResultDto.EquityCurve)-1].OpenTime
		for _, isConflicted := range partConflictedFlags {
			if isConflicted {
				backtestResultDto.Summary.ConflictedCandleCount++
			}
		}

		return backtestResultDto
	}

	wholeResultDto := replayed(inputKCandles, signals, conflictedFlags)
	if !backtestDomain.segments.IsSplit() {
		return wholeResultDto
	}

	// The candles were checked for a bar on either side when they were selected, so the
	// split cannot fail here; should it, the whole replay still stands on its own.
	splitIndex, splitError := backtestDomain.splitIndexOf(inputKCandles)
	if splitError != nil {
		return wholeResultDto
	}

	inSampleResultDto := replayed(
		inputKCandles[:splitIndex], signals[:splitIndex], conflictedFlags[:splitIndex])
	validationResultDto := replayed(
		inputKCandles[splitIndex:], signals[splitIndex:], conflictedFlags[splitIndex:])
	wholeResultDto.ValidationStartTime = backtestDomain.segments.ValidationStartTime()
	wholeResultDto.InSample = &inSampleResultDto
	wholeResultDto.Validation = &validationResultDto

	return wholeResultDto
}
