package service

import (
	"context"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// IndicatorCalculationService is the application layer's only entry point for
// running a user-written indicator script.
type IndicatorCalculationService struct {
	kCandleRepository    domaininterface.IKCandleRepository
	indicatorScriptProxy domaininterface.IIndicatorScriptProxy
	clockProxy           domaininterface.IClockProxy
	maxCandleCount       int
}

func NewIndicatorCalculationService(
	kCandleRepository domaininterface.IKCandleRepository,
	indicatorScriptProxy domaininterface.IIndicatorScriptProxy,
	clockProxy domaininterface.IClockProxy,
	maxCandleCount int,
) *IndicatorCalculationService {
	return &IndicatorCalculationService{
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
		clockProxy:           clockProxy,
		maxCandleCount:       maxCandleCount,
	}
}

// CalculateIndicator runs the script over the requested number of aggregated K
// candles for the trading symbol, taking only buckets that have finished, and
// reports one value per indicator name in the kind the request declared. An empty
// set of names is a valid result.
//
// It reads up to the cut-off the request works out rather than simply the latest
// few, so that the same question asked twice about the same stretch of market
// answers the same thing both times — **as long as that stretch has settled**.
//
// It has not settled at the live edge. The cut-off is worked out from the clock,
// not from what has actually been stored, and ingestion writes a candle a little
// after the five minutes it covers have passed. So at 10:00:30 with one-hour
// buckets, the hour that began at 09:00 counts as finished while the candle for
// 09:55 may still be on its way: that bucket merges eleven of its twelve candles,
// and the same request a few minutes later answers differently.
//
// The alternative — refusing a bucket until it holds every candle it could hold —
// is worse: it cannot tell a market that did not trade from one whose data has not
// arrived, so it would refuse the newest bucket forever on any thin symbol. Naming
// the limit is the honest option; a caller that needs a settled answer names an
// end time in the past, where the guarantee does hold.
func (indicatorCalculationService *IndicatorCalculationService) CalculateIndicator(
	executionContext context.Context, requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
		requestDto,
		indicatorCalculationService.maxCandleCount,
		indicatorCalculationService.clockProxy.Now(),
	)
	if validationError != nil {
		return dto.IndicatorCalculationResultDto{}, validationError
	}

	newestFirstKCandles, findError := indicatorCalculationService.kCandleRepository.FindLatestBefore(
		executionContext,
		calculationDomain.Symbol(),
		calculationDomain.ReadCutoff(),
		calculationDomain.SourceCandleLimit(),
	)
	if findError != nil {
		return dto.IndicatorCalculationResultDto{}, findError
	}

	inputKCandleVos, selectionError := calculationDomain.SelectInputCandles(newestFirstKCandles)
	if selectionError != nil {
		return dto.IndicatorCalculationResultDto{}, selectionError
	}

	indicatorValues, executionError := indicatorCalculationService.indicatorScriptProxy.Execute(
		executionContext, requestDto.Script, calculationDomain.ResultType(), inputKCandleVos)
	if executionError != nil {
		return dto.IndicatorCalculationResultDto{}, executionError
	}

	indicatorValueDtos := make(map[string]dto.IndicatorValueDto, len(indicatorValues))
	for indicatorName, indicatorValue := range indicatorValues {
		indicatorValueDtos[indicatorName] = indicatorValue.ToDto()
	}

	// Where each candle the script saw begins, in the same order the script saw
	// them, so that a caller can put a list of values back where they belong
	// instead of cutting the same grid a second time to find out.
	openTimes := make([]time.Time, 0, len(inputKCandleVos))
	for _, inputKCandleVo := range inputKCandleVos {
		openTimes = append(openTimes, time.Unix(inputKCandleVo.OpenTimeUnixSeconds, 0).UTC())
	}

	return dto.IndicatorCalculationResultDto{
		Symbol:          calculationDomain.Symbol(),
		Interval:        string(calculationDomain.Interval().Value()),
		UsedCandleCount: len(inputKCandleVos),
		OpenTimes:       openTimes,
		ResultType:      string(calculationDomain.ResultType().Value()),
		Values:          indicatorValueDtos,
	}, nil
}
