package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const maxCandleCount = 1000

// calculationNow is the moment every calculation below is asked at. It sits on a
// five-minute edge, so the candle at :15 belongs to a bucket that has finished and
// is read like any other — what is left out is the bucket still running.
var calculationNow = time.Date(2026, 8, 29, 9, 20, 0, 0, time.UTC)

func storedCandleAt(minute int) entities.KCandle {
	return entities.KCandle{
		Symbol:   "BTCUSDT",
		OpenTime: time.Date(2026, 8, 29, 9, minute, 0, 0, time.UTC),
		Close:    decimal.RequireFromString("100"),
	}
}

// newestFirst builds candles in the order storage returns them: newest first.
func newestFirst(minutes ...int) []entities.KCandle {
	kCandles := make([]entities.KCandle, 0, len(minutes))
	for _, minute := range minutes {
		kCandles = append(kCandles, storedCandleAt(minute))
	}
	return kCandles
}

func calculationRequest(symbol string, candleCount int) dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol: symbol, CandleCount: candleCount, Script: "the script",
	}
}

func calculationRequestOf(
	symbol string, candleCount int, resultType string,
) dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol: symbol, CandleCount: candleCount, Script: "the script", ResultType: resultType,
	}
}

// oneMinuteCutoff is where a read stops when nothing coarser was declared: the
// start of the minute still running, which at 09:20 is 09:20 itself.
var oneMinuteCutoff = calculationNow

type calculationUnderTest struct {
	indicatorCalculationService *service.IndicatorCalculationService
	kCandleRepository           *mocks.MockIKCandleRepository
	indicatorScriptProxy        *mocks.MockIIndicatorScriptProxy
}

func newCalculationUnderTest(t *testing.T) calculationUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(calculationNow).AnyTimes()

	return calculationUnderTest{
		indicatorCalculationService: service.NewIndicatorCalculationService(
			kCandleRepository, indicatorScriptProxy, clockProxy, maxCandleCount),
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
	}
}

func TestCalculateIndicator(t *testing.T) {
	t.Run("asks storage for one bucket more than requested, up to the cut-off", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).
			Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(), calculationRequest("BTCUSDT", 3))

		assert.NoError(t, err)
	})

	t.Run("hands the script the requested candles oldest first, from finished buckets", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), "the script", gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context,
				script string,
				resultType domains.IndicatorResultTypeDomain,
				kCandleVos []vo.KCandleVo,
				_ domains.StrategyParametersDomain,
			) (map[string]vo.IndicatorValueVo, error) {
				assert.Len(t, kCandleVos, 3)
				assert.Equal(t, storedCandleAt(5).OpenTime.Unix(), kCandleVos[0].OpenTimeUnixSeconds)
				assert.Equal(t, storedCandleAt(10).OpenTime.Unix(), kCandleVos[1].OpenTimeUnixSeconds)
				assert.Equal(t, storedCandleAt(15).OpenTime.Unix(), kCandleVos[2].OpenTimeUnixSeconds)
				return map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil
			})

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(), calculationRequest("BTCUSDT", 3))

		assert.NoError(t, err)
	})

	t.Run("reports the indicator values and how many candles were used", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{
				"high": {Numbers: []float64{120}}, "low": {Numbers: []float64{100}},
			}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequest("BTCUSDT", 3))

		assert.NoError(t, err)
		assert.Equal(t, "BTCUSDT", resultDto.Symbol)
		assert.Equal(t, 3, resultDto.UsedCandleCount)
		assert.Equal(t, []float64{120}, resultDto.Values["high"].Numbers)
		assert.Equal(t, []float64{100}, resultDto.Values["low"].Numbers)
	})

	t.Run("treats an empty set of indicator values as a success", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 2).Return(newestFirst(5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequest("BTCUSDT", 1))

		assert.NoError(t, err)
		assert.Empty(t, resultDto.Values)
		assert.Equal(t, 1, resultDto.UsedCandleCount)
	})

	t.Run("never reaches storage when the request breaks a rule", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(), calculationRequest("BTCUSDT", 0))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "計算根數必須大於零")
	})

	t.Run("never reaches storage when too many candles are requested", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequest("BTCUSDT", maxCandleCount+1))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "超過單次可用的最大根數")
	})

	t.Run("answers over a short stretch, reporting both counts", func(t *testing.T) {
		// Thirty were asked for and one is stored. The run is not refused: it answers
		// over the one, and says so by putting the two counts side by side. Without
		// the pair, a caller cannot tell a short answer from a full one.
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 31).
			Return(newestFirst(0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(
			t.Context(), calculationRequest("BTCUSDT", 30))

		require.NoError(t, err)
		assert.Equal(t, 30, resultDto.RequiredCandleCount, "整段填滿要三十根")
		assert.Equal(t, 1, resultDto.UsedCandleCount, "手上只有一根")
		assert.Len(t, resultDto.OpenTimes, 1, "起始時間的個數等於實際採用根數")
	})

	t.Run("reports both counts as the same number when the stretch is all there", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).
			Return(newestFirst(10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(
			t.Context(), calculationRequest("BTCUSDT", 3))

		require.NoError(t, err)
		assert.Equal(t, 3, resultDto.RequiredCandleCount)
		assert.Equal(t, 3, resultDto.UsedCandleCount)
	})

	t.Run("never runs the script over a stretch too thin to yield one value", func(t *testing.T) {
		// Nothing stored at all. There is no value to be had, so this stays a refusal
		// — and the script is never reached, because there is nothing to run it over.
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 31).
			Return(nil, nil)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(
			t.Context(), calculationRequest("BTCUSDT", 30))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationCandleCoverageTooThin)
		availableCandleCount, minimumCandleCount, isTooThin :=
			domains.CandleCoverageShortfall(err)
		require.True(t, isTooThin)
		assert.Equal(t, 0, availableCandleCount)
		assert.Equal(t, 1, minimumCandleCount)
	})

	t.Run("an algorithm needing more than it declared fails as an algorithm", func(t *testing.T) {
		// Nothing is declared, so the floor is one candle and three is answerable —
		// but the algorithm actually reaches back over twenty and blows up on three.
		// That has to come back as the algorithm's problem: reported as a shortfall it
		// would send somebody to fetch more history for a script that will fail on any
		// amount, and the system has no way to know what a script reaches for.
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 31).
			Return(newestFirst(10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context,
				_ string,
				_ domains.IndicatorResultTypeDomain,
				kCandleVos []vo.KCandleVo,
				_ domains.StrategyParametersDomain,
			) (map[string]vo.IndicatorValueVo, error) {
				assert.Len(t, kCandleVos, 3, "手上那三根照樣交給算式")

				return nil, domains.ErrIndicatorScriptFailed
			})

		_, err := fixture.indicatorCalculationService.CalculateIndicator(
			t.Context(), calculationRequest("BTCUSDT", 30))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.NotErrorIs(t, err, domains.ErrIndicatorCalculationCandleCoverageTooThin,
			"不是根數不足——系統不猜算式需要幾根")
	})

	t.Run("reports a storage failure as neither a request nor a script problem", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		storageFailure := errors.New("storage unreachable")
		fixture.kCandleRepository.EXPECT().FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).Return(nil, storageFailure)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequest("BTCUSDT", 3))

		assert.ErrorIs(t, err, storageFailure)
		assert.NotErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.NotErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Empty(t, resultDto.Values)
	})

	t.Run("reports a script failure without any partial result", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, domains.ErrIndicatorScriptFailed)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequest("BTCUSDT", 3))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Empty(t, resultDto.Values)
		assert.Equal(t, 0, resultDto.UsedCandleCount)
	})
}

func TestCalculateIndicatorCarriesTheDeclaredResultType(t *testing.T) {
	t.Run("hands the script runner the kind that was declared", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context,
				script string,
				resultType domains.IndicatorResultTypeDomain,
				kCandleVos []vo.KCandleVo,
				_ domains.StrategyParametersDomain,
			) (map[string]vo.IndicatorValueVo, error) {
				assert.Equal(t, vo.IndicatorResultTypeFloatList, resultType.Value())
				return map[string]vo.IndicatorValueVo{}, nil
			})

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequestOf("BTCUSDT", 3, "floatList"))

		assert.NoError(t, err)
	})

	t.Run("reports the kind alongside the values", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{
				"red": {IsList: true, Booleans: []bool{true, false}},
			}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequestOf("BTCUSDT", 3, "boolList"))

		assert.NoError(t, err)
		assert.Equal(t, "boolList", resultDto.ResultType)
		assert.True(t, resultDto.Values["red"].IsList)
		assert.Equal(t, []bool{true, false}, resultDto.Values["red"].Booleans)
	})

	t.Run("reports the signal itself, with no indicator name, under the signal kind", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context,
				_ string,
				resultType domains.IndicatorResultTypeDomain,
				_ []vo.KCandleVo,
				_ domains.StrategyParametersDomain,
			) (map[string]vo.IndicatorValueVo, error) {
				assert.True(t, resultType.IsSignal())
				return map[string]vo.IndicatorValueVo{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}}, nil
			})

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequestOf("BTCUSDT", 3, "signal"))

		assert.NoError(t, err)
		assert.Equal(t, "signal", resultDto.ResultType)
		assert.Equal(t, "buy", resultDto.Signal)
		assert.Empty(t, resultDto.Values)
	})

	t.Run("reports one number per indicator when nothing was declared", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 2).Return(newestFirst(5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequest("BTCUSDT", 1))

		assert.NoError(t, err)
		assert.Equal(t, "float", resultDto.ResultType)
	})

	t.Run("never reaches storage when the declared kind is not on offer", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequestOf("BTCUSDT", 3, "string"))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "指標值種類只能是")
	})
}

func TestCalculateIndicatorKeepsEveryOtherRuleWhateverTheKindIs(t *testing.T) {
	t.Run("a candle count of zero is refused just the same", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequestOf("BTCUSDT", 0, "floatList"))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "計算根數必須大於零")
	})

	t.Run("a short stretch is answered over just the same", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 31).
			Return(newestFirst(10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequestOf("BTCUSDT", 30, "bool"))

		require.NoError(t, err)
		assert.Equal(t, 30, resultDto.RequiredCandleCount)
		assert.Equal(t, 3, resultDto.UsedCandleCount)
	})

	t.Run("a stretch too thin to yield one value is refused just the same", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 31).
			Return(nil, nil)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequestOf("BTCUSDT", 30, "bool"))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationCandleCoverageTooThin)
	})

	t.Run("the candles handed to the script are chosen the same way", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context,
				script string,
				resultType domains.IndicatorResultTypeDomain,
				kCandleVos []vo.KCandleVo,
				_ domains.StrategyParametersDomain,
			) (map[string]vo.IndicatorValueVo, error) {
				assert.Len(t, kCandleVos, 3)
				assert.Equal(t, storedCandleAt(5).OpenTime.Unix(), kCandleVos[0].OpenTimeUnixSeconds)
				assert.Equal(t, storedCandleAt(10).OpenTime.Unix(), kCandleVos[1].OpenTimeUnixSeconds)
				assert.Equal(t, storedCandleAt(15).OpenTime.Unix(), kCandleVos[2].OpenTimeUnixSeconds)
				return map[string]vo.IndicatorValueVo{}, nil
			})

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequestOf("BTCUSDT", 3, "floatList"))

		assert.NoError(t, err)
		assert.Equal(t, 3, resultDto.UsedCandleCount)
	})
}

func TestCalculateIndicatorReadsAtTheCoarsenessItWasAsked(t *testing.T) {
	t.Run("a coarser interval stops before the bucket still running", func(t *testing.T) {
		// At 09:20 the hour that began at 09:00 is twenty minutes old. Reading stops
		// at 09:00, so it is not read at all — its twenty candles would otherwise be
		// merged into an hour that keeps changing.
		fixture := newCalculationUnderTest(t)
		hourCutoff := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", hourCutoff, 180).
			Return([]entities.KCandle{}, nil)

		requestDto := calculationRequest("BTCUSDT", 2)
		requestDto.AggregationInterval = "1h"

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(), requestDto)

		// The read is what this test is about; coming back empty then refuses, which
		// is the rule the case above already covers.
		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
	})

	t.Run("an end time already past is read up to, not up to now", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		pastCutoff := time.Date(2025, 3, 1, 14, 0, 0, 0, time.UTC)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", pastCutoff, 180).
			Return([]entities.KCandle{}, nil)

		requestDto := calculationRequest("BTCUSDT", 2)
		requestDto.AggregationInterval = "1h"
		requestDto.EndTime = time.Date(2025, 3, 1, 14, 30, 0, 0, time.UTC)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(), requestDto)

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
	})

	t.Run("an end time that has not arrived is read up to now", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).
			Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{}, nil)

		requestDto := calculationRequest("BTCUSDT", 3)
		requestDto.EndTime = calculationNow.Add(24 * time.Hour)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(), requestDto)

		assert.NoError(t, err)
	})
}

func TestCalculateIndicatorSaysWhichStretchOfMarketItRead(t *testing.T) {
	// A caller putting a list of values back onto a chart has to know which candle
	// each one belongs to. Answering it here is what stops the caller cutting the
	// same grid a second time and landing one bucket out.
	t.Run("names where each candle the script saw begins, earliest first", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).
			Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{
				"ma": {IsList: true, Numbers: []float64{101, 102, 103}},
			}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequestOf("BTCUSDT", 3, "floatList"))

		assert.NoError(t, err)
		assert.Equal(t, []time.Time{
			storedCandleAt(5).OpenTime, storedCandleAt(10).OpenTime, storedCandleAt(15).OpenTime,
		}, resultDto.OpenTimes)
		assert.Len(t, resultDto.Values["ma"].Numbers, len(resultDto.OpenTimes),
			"第 n 個值對應第 n 個起始時間，所以兩者一樣長")
	})

	t.Run("names them even when the kind is a single number", func(t *testing.T) {
		// They describe what was read, not what came out, so how many values there
		// are has nothing to do with it.
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).
			Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequest("BTCUSDT", 3))

		assert.NoError(t, err)
		assert.Len(t, resultDto.OpenTimes, 3)
	})

	t.Run("names them even when the script produced nothing at all", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", oneMinuteCutoff, 4).
			Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequest("BTCUSDT", 3))

		assert.NoError(t, err)
		assert.Empty(t, resultDto.Values)
		assert.Len(t, resultDto.OpenTimes, 3)
		assert.Equal(t, 3, resultDto.UsedCandleCount)
	})

	t.Run("names the coarseness actually used, declared or not", func(t *testing.T) {
		testCases := []struct {
			name             string
			declaredInterval string
			expectedInterval string
		}{
			{name: "declared", declaredInterval: "1h", expectedInterval: "1h"},
			{name: "left out", declaredInterval: "", expectedInterval: "1m"},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newCalculationUnderTest(t)
				fixture.kCandleRepository.EXPECT().
					FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
					Return(newestFirst(15, 10, 5, 0), nil)
				fixture.indicatorScriptProxy.EXPECT().
					Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(map[string]vo.IndicatorValueVo{}, nil)

				requestDto := calculationRequest("BTCUSDT", 1)
				requestDto.AggregationInterval = testCase.declaredInterval

				resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(
					t.Context(), requestDto)

				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedInterval, resultDto.Interval)
			})
		}
	})
}
