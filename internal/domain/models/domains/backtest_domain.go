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

// minimumBacktestKCandleCount is two because a single candle gives a decision no later price to be judged at.
const minimumBacktestKCandleCount = 2

// BacktestDomain validates what may be replayed; running the replay belongs to BacktestSimulationDomain.
type BacktestDomain struct {
	symbol         string
	interval       AggregationIntervalDomain
	parameters     StrategyScriptParametersDomain
	initialCapital decimal.Decimal
	positionTerms  BacktestPositionTermsDomain
	fillTiming     BacktestFillTimingDomain
	segments       BacktestSegmentsDomain
	startTime      time.Time
	// readCutoff excludes the still-running last bucket: only buckets opened strictly before it are replayed.
	readCutoff time.Time
}

// effectiveEndTime treats a zero or future end as now, since the market cannot be read past the present.
func effectiveEndTime(declaredEndTime time.Time, now time.Time) time.Time {
	if declaredEndTime.IsZero() || declaredEndTime.After(now) {
		return now
	}

	return declaredEndTime
}

// NewBacktestDomain takes now as a parameter so validation stays deterministic.
func NewBacktestDomain(
	requestDto dto.BacktestRequestDto, maxCandleCount int, now time.Time,
) (BacktestDomain, error) {
	tradingSymbol, symbolError := NewTradingSymbolDomain(requestDto.Symbol)
	if symbolError != nil {
		return BacktestDomain{}, fmt.Errorf("%w: %w", ErrBacktestValidation, symbolError)
	}

	// Spot-only is checked first because it refuses a different kind of replay entirely.
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
		// Only a bad figure names a field; an unknown mode is answered by listing the valid modes.
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
		return BacktestDomain{}, BacktestValidationFailure(
			BacktestExitLevelsField, exitLevelsError.Error())
	}

	transactionCosts, transactionCostsError := NewBacktestTransactionCostsDomain(
		requestDto.EntryCostPercentage, requestDto.ExitCostPercentage)
	if transactionCostsError != nil {
		return BacktestDomain{}, BacktestValidationFailure(
			BacktestTransactionCostsField, transactionCostsError.Error())
	}

	// A percentage whose entry cost is unaffordable would never open a trade, so refuse it on the sizing field rather than return an empty report.
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
	// The bucket the stretch ends in is still running, so it is excluded, matching the indicator calculation rule.
	readCutoff := interval.BucketStart(endTime)

	// An inverted stretch or one shorter than a bucket gets the same not-enough-candles refusal.
	if !readCutoff.After(startTime) {
		return BacktestDomain{}, notEnoughKCandlesForBacktest(0)
	}

	// The validation start is judged only once the stretch is known to be non-empty.
	segments, segmentsError := NewBacktestSegmentsDomain(requestDto.ValidationStartTime, startTime, endTime)
	if segmentsError != nil {
		return BacktestDomain{}, segmentsError
	}

	bucketCount := interval.BucketCount(startTime, readCutoff)
	if bucketCount > maxCandleCount {
		return BacktestDomain{}, BacktestValidationFailure(
			BacktestTimeRangeField,
			fmt.Sprintf("這一段以這個彙總刻度要用到 %d 根，超過一次重演可用的最大根數（最多 %d 根）",
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

// Parameters already carry each declared parameter's resolved value.
func (backtestDomain BacktestDomain) Parameters() StrategyScriptParametersDomain {
	return backtestDomain.parameters
}

// ResultType is always the signal kind, since a replay acts on an opinion per candle.
func (backtestDomain BacktestDomain) ResultType() IndicatorResultTypeDomain {
	return IndicatorResultTypeDomain{value: vo.IndicatorResultTypeSignal}
}

// KCandleQuery reads up to and including the cut-off (one harmless extra candle); the replay itself excludes it.
func (backtestDomain BacktestDomain) KCandleQuery() KCandleQueryDomain {
	return KCandleQueryDomain{
		symbol:    backtestDomain.symbol,
		startTime: backtestDomain.startTime,
		endTime:   backtestDomain.readCutoff,
	}
}

// SourceCandleLimit allows one spare bucket so the read is never cut short yet stays bounded.
func (backtestDomain BacktestDomain) SourceCandleLimit() int {
	return backtestDomain.interval.SourceCandleCount(
		backtestDomain.interval.BucketCount(backtestDomain.startTime, backtestDomain.readCutoff) +
			spareBucketCount)
}

// SelectInputCandles aggregates into finished buckets, oldest first, refusing fewer than two.
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

	// Refuse a split lacking a bar on either side before any script runs.
	if _, splitError := backtestDomain.splitIndexOf(inputKCandleVos); splitError != nil {
		return nil, splitError
	}

	return inputKCandleVos, nil
}

// splitIndexOf returns the full length for an unsplit replay.
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

// ReplayOver replays the whole stretch and, when split, each part again from the initial capital, using the same signals throughout.
// conflictedFlags marks candles where both of a trading strategy's trees held; strategy script replays pass nil.
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

	// Already checked in SelectInputCandles; if it fails anyway, the whole replay still stands.
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
