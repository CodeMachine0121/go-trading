package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// spareBucketCount is the extra bucket every read reaches over, so the earliest bucket a candle-count limit may cut in half is never among those handed over.
const spareBucketCount = 1

// IndicatorCalculationDomain owns which K candles a script sees: interval, count, cutoff, and never a still-running bucket.
type IndicatorCalculationDomain struct {
	symbol string
	// candleCount is the finished buckets needed to fill the observation window; a short stretch yields fewer, never an error.
	candleCount int
	parameters  StrategyScriptParametersDomain
	resultType  IndicatorResultTypeDomain
	interval    AggregationIntervalDomain
	// endTime is already settled: never zero and never in the future.
	endTime time.Time
}

// NewIndicatorCalculationDomain validates the request and sizes the candle count; market and now are passed in so the result depends only on arguments.
func NewIndicatorCalculationDomain(
	requestDto dto.IndicatorCalculationRequestDto,
	marketDomain MarketDomain,
	maxCandleCount int,
	now time.Time,
) (IndicatorCalculationDomain, error) {
	tradingSymbol, symbolError := NewTradingSymbolDomain(requestDto.Symbol)
	if symbolError != nil {
		return IndicatorCalculationDomain{},
			fmt.Errorf("%w: %w", ErrIndicatorCalculationValidation, symbolError)
	}

	declaredParameters, parametersError := NewStrategyScriptParametersDomain(requestDto.Parameters)
	if parametersError != nil {
		return IndicatorCalculationDomain{}, fmt.Errorf(
			"%w: %w", ErrIndicatorCalculationValidation, parametersError)
	}

	parameters, applyError := declaredParameters.Applying(requestDto.ParameterValues)
	if applyError != nil {
		return IndicatorCalculationDomain{}, fmt.Errorf(
			"%w: %w", ErrIndicatorCalculationValidation, applyError)
	}

	interval, intervalError := NewAggregationIntervalDomain(requestDto.AggregationInterval)
	if intervalError != nil {
		return IndicatorCalculationDomain{}, fmt.Errorf(
			"%w: %w", ErrIndicatorCalculationValidation, intervalError)
	}

	observationWindow, windowError := NewObservationWindowDomain(
		requestDto.StartTime, requestDto.EndTime, now)
	if windowError != nil {
		return IndicatorCalculationDomain{}, fmt.Errorf(
			"%w: %w", ErrIndicatorCalculationValidation, windowError)
	}

	// Asked directly rather than read off a rounded bucket count, which would misread a sub-bucket stretch as closed.
	if !marketDomain.HoldsTrading(observationWindow.StartTime(), observationWindow.EndTime()) {
		return IndicatorCalculationDomain{}, ObservationWindowHoldsNoTrading(marketDomain.Value())
	}

	// Slots in the window plus the look-back minus the one they share; the extra candles deliberately come from before the window.
	inputCandleCount := interval.TradingSlotCount(
		marketDomain, observationWindow.StartTime(), observationWindow.EndTime()) +
		max(0, parameters.MaximumLookbackCount()-1)

	// The ceiling counts aggregated candles actually fed to the algorithm, look-back included.
	if inputCandleCount > maxCandleCount {
		return IndicatorCalculationDomain{}, CandleCountExceeded(inputCandleCount, maxCandleCount)
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
		endTime:     observationWindow.EndTime(),
	}, nil
}

func (indicatorCalculationDomain IndicatorCalculationDomain) Symbol() string {
	return indicatorCalculationDomain.symbol
}

// ResultType is the already-parsed declaration, so it is interpreted only once.
func (indicatorCalculationDomain IndicatorCalculationDomain) ResultType() IndicatorResultTypeDomain {
	return indicatorCalculationDomain.resultType
}

func (indicatorCalculationDomain IndicatorCalculationDomain) Interval() AggregationIntervalDomain {
	return indicatorCalculationDomain.interval
}

// ReadCutoff is the start of the bucket containing the end time; only candles opened strictly before it are read, so a half-formed bucket is never used, even for past end times.
func (indicatorCalculationDomain IndicatorCalculationDomain) ReadCutoff() time.Time {
	return indicatorCalculationDomain.interval.BucketStart(indicatorCalculationDomain.endTime)
}

// SourceCandleLimit is the stored-candle count for the requested buckets plus one spare; reading by count rather than time makes gaps free.
func (indicatorCalculationDomain IndicatorCalculationDomain) SourceCandleLimit() int {
	return indicatorCalculationDomain.interval.SourceCandleCount(
		indicatorCalculationDomain.candleCount + spareBucketCount)
}

// SelectInputCandles merges newest-first candles into finished buckets and returns up to candleCount of the latest, oldest first; a short history answers with fewer values.
// The earliest bucket may be partial because backfill starts mid-bucket, and it is deliberately kept since a thin bucket cannot be told apart from a truncated one.
func (indicatorCalculationDomain IndicatorCalculationDomain) SelectInputCandles(
	newestFirstKCandles []entities.KCandle,
) ([]vo.KCandleVo, error) {
	buckets := NewKCandleSeriesDomain(
		indicatorCalculationDomain.symbol,
		indicatorCalculationDomain.interval,
		newestFirstKCandles,
	).Buckets()

	usedCandleCount, coverageError := indicatorCalculationDomain.usableBucketCount(len(buckets))
	if coverageError != nil {
		return nil, coverageError
	}
	latestBuckets := buckets[len(buckets)-usedCandleCount:]

	oldestFirstKCandleVos := make([]vo.KCandleVo, 0, usedCandleCount)
	for _, bucket := range latestBuckets {
		oldestFirstKCandleVos = append(oldestFirstKCandleVos, bucket.ToVo())
	}

	return oldestFirstKCandleVos, nil
}

// SelectContractInput applies SelectInputCandles' rules to contract candles, returning an alignment still to be completed with funding and positioning.
func (indicatorCalculationDomain IndicatorCalculationDomain) SelectContractInput(
	newestFirstKCandleContracts []entities.KCandleContract,
) (ContractKCandleAlignmentDomain, error) {
	buckets := NewKCandleContractSeriesDomain(
		indicatorCalculationDomain.symbol,
		indicatorCalculationDomain.interval,
		newestFirstKCandleContracts,
	).ToDto().KCandles

	usedCandleCount, coverageError := indicatorCalculationDomain.usableBucketCount(len(buckets))
	if coverageError != nil {
		return ContractKCandleAlignmentDomain{}, coverageError
	}

	return newContractKCandleAlignmentDomain(
		indicatorCalculationDomain.interval, buckets[len(buckets)-usedCandleCount:]), nil
}

// usableBucketCount returns the latest candleCount buckets, refusing when fewer than the declared look-back (at least one) are available.
// Only declared look-backs are considered; an undeclared hard-coded period surfaces as a script failure.
func (indicatorCalculationDomain IndicatorCalculationDomain) usableBucketCount(availableBucketCount int) (int, error) {
	minimumCandleCount := max(1, indicatorCalculationDomain.parameters.MaximumLookbackCount())
	if availableBucketCount < minimumCandleCount {
		return 0, CandleCoverageTooThin(availableBucketCount, minimumCandleCount)
	}

	return min(indicatorCalculationDomain.candleCount, availableBucketCount), nil
}

// ToResultDto builds the shared spot/contract result shape; a signal result carries the signal itself instead of named values.
func (indicatorCalculationDomain IndicatorCalculationDomain) ToResultDto(
	openTimes []time.Time, indicatorValues map[string]vo.IndicatorValueVo,
) dto.IndicatorCalculationResultDto {
	resultDto := dto.IndicatorCalculationResultDto{
		Symbol:              indicatorCalculationDomain.symbol,
		Interval:            string(indicatorCalculationDomain.interval.Value()),
		RequiredCandleCount: indicatorCalculationDomain.candleCount,
		UsedCandleCount:     len(openTimes),
		OpenTimes:           openTimes,
		ResultType:          string(indicatorCalculationDomain.resultType.Value()),
	}

	if indicatorCalculationDomain.resultType.IsSignal() {
		resultDto.Signal = string(NewSignalDomain(indicatorValues).Value())
		return resultDto
	}

	indicatorValueDtos := make(map[string]dto.IndicatorValueDto, len(indicatorValues))
	for indicatorName, indicatorValue := range indicatorValues {
		indicatorValueDtos[indicatorName] = indicatorValue.ToDto()
	}
	resultDto.Values = indicatorValueDtos

	return resultDto
}

// CandleCount is the buckets a full answer would need, exposed so callers can report a short stretch without re-deriving it.
func (indicatorCalculationDomain IndicatorCalculationDomain) CandleCount() int {
	return indicatorCalculationDomain.candleCount
}

// Parameters carry each declared knob's settled value, from the run or its default.
func (indicatorCalculationDomain IndicatorCalculationDomain) Parameters() StrategyScriptParametersDomain {
	return indicatorCalculationDomain.parameters
}
