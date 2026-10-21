package service_test

import (
	"context"
	"errors"
	"slices"
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

// calculationNow sits on a five-minute edge, so only the bucket still running is left out.
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

// calculationRequest asks about exactly candleCount one-minute slots; every symbol here trades round the clock.
func calculationRequest(symbol string, candleCount int) dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol:    symbol,
		StartTime: calculationNow.Add(-time.Duration(candleCount) * time.Minute),
		Script:    "the script",
	}
}

func calculationRequestOf(
	symbol string, candleCount int, resultType string,
) dto.IndicatorCalculationRequestDto {
	requestDto := calculationRequest(symbol, candleCount)
	requestDto.ResultType = resultType

	return requestDto
}

// oneMinuteCutoff is the start of the minute still running, where a read stops when nothing coarser is declared.
var oneMinuteCutoff = calculationNow

type calculationUnderTest struct {
	indicatorCalculationService *service.IndicatorCalculationService
	kCandleRepository           *mocks.MockIKCandleRepository
	kCandleSummaryDayRepository *mocks.MockIKCandleSummaryDayRepository
	tradingSymbolRepository     *mocks.MockITradingSymbolRepository
	indicatorScriptProxy        *mocks.MockIIndicatorScriptProxy
}

func newCalculationUnderTest(t *testing.T) calculationUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	kCandleSummaryDayRepository := mocks.NewMockIKCandleSummaryDayRepository(controller)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	// Symbols default to the round-the-clock market, which every stretch in this file assumes.
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(calculationNow).AnyTimes()

	return calculationUnderTest{
		indicatorCalculationService: service.NewIndicatorCalculationService(
			kCandleRepository, kCandleSummaryDayRepository, tradingSymbolRepository, indicatorScriptProxy, clockProxy,
			marketCatalog(), maxCandleCount),
		kCandleRepository:           kCandleRepository,
		kCandleSummaryDayRepository: kCandleSummaryDayRepository,
		tradingSymbolRepository:     tradingSymbolRepository,
		indicatorScriptProxy:        indicatorScriptProxy,
	}
}

// holdKCandles stubs storage holding exactly these minutes, with the given days already summarized from them, so every read path answers from the same market.
func (fixture calculationUnderTest) holdKCandles(kCandles []entities.KCandle, summarizedDays ...time.Time) {
	fixture.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, cutoffTime time.Time, limit int) ([]entities.KCandle, error) {
			newestFirstHeld := []entities.KCandle{}
			for _, kCandle := range kCandles {
				if kCandle.OpenTime.Before(cutoffTime) {
					newestFirstHeld = append(newestFirstHeld, kCandle)
				}
			}
			slices.SortFunc(newestFirstHeld, func(former, latter entities.KCandle) int {
				return latter.OpenTime.Compare(former.OpenTime)
			})
			return newestFirstHeld[:min(limit, len(newestFirstHeld))], nil
		}).AnyTimes()
	fixture.kCandleRepository.EXPECT().
		FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, query domains.KCandleQueryDomain, limit int) ([]entities.KCandle, error) {
			earliestFirstHeld := []entities.KCandle{}
			for _, kCandle := range kCandles {
				if !kCandle.OpenTime.Before(query.StartTime()) && !kCandle.OpenTime.After(query.EndTime()) {
					earliestFirstHeld = append(earliestFirstHeld, kCandle)
				}
			}
			slices.SortFunc(earliestFirstHeld, func(former, latter entities.KCandle) int {
				return former.OpenTime.Compare(latter.OpenTime)
			})
			return earliestFirstHeld[:min(limit, len(earliestFirstHeld))], nil
		}).AnyTimes()
	fixture.kCandleSummaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, symbol string, interval vo.AggregationIntervalVo, startTime time.Time, endTime time.Time,
		) ([]entities.KCandleSummaryDay, error) {
			summaryDays := []entities.KCandleSummaryDay{}
			for _, day := range summarizedDays {
				if day.Before(startTime.Truncate(domains.KCandleSummaryDayLength)) || !day.Before(endTime) {
					continue
				}
				dayMinutes := []entities.KCandle{}
				for _, kCandle := range kCandles {
					if !kCandle.OpenTime.Before(day) && kCandle.OpenTime.Before(day.Add(domains.KCandleSummaryDayLength)) {
						dayMinutes = append(dayMinutes, kCandle)
					}
				}
				summaryDay := domains.NewKCandleSummaryDayDomain(symbol, day, 1, dayMinutes).ToEntity()
				inRange := []entities.KCandleSummary{}
				for _, summary := range summaryDay.Summaries {
					if summary.Interval == string(interval) && !summary.OpenTime.Before(startTime) && summary.OpenTime.Before(endTime) {
						inRange = append(inRange, summary)
					}
				}
				summaryDay.Summaries = inRange
				summaryDays = append(summaryDays, summaryDay)
			}
			return summaryDays, nil
		}).AnyTimes()
	fixture.kCandleSummaryDayRepository.EXPECT().
		SaveSummarized(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
}

// marketCatalog holds the round-the-clock market and Taiwan's 09:00–13:30 Taipei, Monday–Friday session.
func marketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
		vo.MarketTaiwanStock: {
			TradingSession: vo.TradingSessionVo{
				Location:   time.FixedZone("Asia/Taipei", 8*60*60),
				DailyStart: 9 * time.Hour,
				DailyEnd:   13*time.Hour + 30*time.Minute,
				Weekdays: []time.Weekday{
					time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
				},
			},
		},
	})
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
				_ domains.StrategyScriptParametersDomain,
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
		assert.Contains(t, err.Error(), "起點必須早於終點")
	})

	t.Run("never reaches storage when too many candles are requested", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequest("BTCUSDT", maxCandleCount+1))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "超過單次可用的最大根數")
	})

	t.Run("answers over a short stretch, reporting both counts", func(t *testing.T) {
		// Thirty asked, one stored: it answers over the one and reports both counts so a short answer is distinguishable from a full one.
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
		// Nothing stored means no value is possible, so it is refused without reaching the script.
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
		// The script reaches back further than declared and fails on three candles; that must surface as an algorithm error, not a shortfall inviting more history.
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
				_ domains.StrategyScriptParametersDomain,
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
				_ domains.StrategyScriptParametersDomain,
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
				_ domains.StrategyScriptParametersDomain,
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
	t.Run("a stretch of no length is refused just the same", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(),
			calculationRequestOf("BTCUSDT", 0, "floatList"))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "起點必須早於終點")
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
				_ domains.StrategyScriptParametersDomain,
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
		// At 09:20 the 09:00 hour is still running, so the read stops at 09:00 rather than merging a changing hour.
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, query domains.KCandleQueryDomain, _ int) ([]entities.KCandle, error) {
				assert.Equal(t, time.Date(2026, 8, 29, 7, 0, 0, 0, time.UTC), query.StartTime())
				assert.Equal(t, time.Date(2026, 8, 29, 8, 59, 59, 999999000, time.UTC), query.EndTime())
				return []entities.KCandle{}, nil
			})
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", time.Date(2026, 8, 29, 7, 0, 0, 0, time.UTC), 1).
			Return([]entities.KCandle{}, nil)

		requestDto := calculationRequest("BTCUSDT", 2)
		requestDto.AggregationInterval = "1h"
		// Two hours of market is two hourly slots.
		requestDto.StartTime = calculationNow.Add(-2 * time.Hour)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(), requestDto)

		// Only the read matters here; the refusal on an empty result is covered above.
		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
	})

	t.Run("an end time already past is read up to, not up to now", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		pastCutoff := time.Date(2025, 3, 1, 14, 0, 0, 0, time.UTC)
		fixture.kCandleSummaryDayRepository.EXPECT().
			FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneHour, pastCutoff.Add(-2*time.Hour), pastCutoff).
			Return(nil, nil)
		fixture.holdKCandles(nil)

		requestDto := calculationRequest("BTCUSDT", 2)
		requestDto.AggregationInterval = "1h"
		requestDto.EndTime = time.Date(2025, 3, 1, 14, 30, 0, 0, time.UTC)
		requestDto.StartTime = requestDto.EndTime.Add(-2 * time.Hour)

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
	// Callers plotting values need each candle's start, so the service answers it rather than letting them recut the grid and land one bucket out.
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
		// Open times describe what was read, not how many values came out.
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
				fixture.holdKCandles(append(newestFirst(15, 10, 5, 0), entities.KCandle{
					Symbol: "BTCUSDT", OpenTime: time.Date(2026, 8, 29, 8, 30, 0, 0, time.UTC), Close: decimal.RequireFromString("100"),
				}))
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

// 服務把交易標的換成它所屬的市場，計算才問得出「這一段時間有多少市場」。
func TestCalculateIndicatorSizesTheReadByTheSymbolsOwnMarket(t *testing.T) {
	// 台北 09:00–13:30 的那一天，用世界標準時間說是 01:00–05:30。
	sessionStart := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	sessionEnd := time.Date(2026, 9, 7, 5, 30, 0, 0, time.UTC)
	askedAt := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)

	buildFixture := func(t *testing.T, market vo.MarketVo) calculationUnderTest {
		t.Helper()

		controller := gomock.NewController(t)
		kCandleRepository := mocks.NewMockIKCandleRepository(controller)
		tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
		tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").
			Return(entities.TradingSymbol{Symbol: "2330", Market: string(market)}, true, nil)
		indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
		clockProxy := mocks.NewMockIClockProxy(controller)
		clockProxy.EXPECT().Now().Return(askedAt).AnyTimes()

		return calculationUnderTest{
			indicatorCalculationService: service.NewIndicatorCalculationService(
				kCandleRepository, mocks.NewMockIKCandleSummaryDayRepository(controller), tradingSymbolRepository, indicatorScriptProxy, clockProxy,
				marketCatalog(), maxCandleCount),
			kCandleRepository:       kCandleRepository,
			tradingSymbolRepository: tradingSymbolRepository,
			indicatorScriptProxy:    indicatorScriptProxy,
		}
	}

	requestOver := func(startTime time.Time, endTime time.Time) dto.IndicatorCalculationRequestDto {
		return dto.IndicatorCalculationRequestDto{
			Symbol:              "2330",
			AggregationInterval: "5m",
			StartTime:           startTime,
			EndTime:             endTime,
			Script:              "the script",
		}
	}

	t.Run("a whole day of a market that shuts asks for one session of slots", func(t *testing.T) {
		fixture := buildFixture(t, vo.MarketTaiwanStock)
		// 54 格加多讀的一格，五分鐘刻度下一格五根 → 275 根原始 K 線。
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "2330", sessionEnd, 275).
			Return([]entities.KCandle{}, nil)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(
			t.Context(), requestOver(sessionEnd.Add(-24*time.Hour), sessionEnd))

		// 讀到什麼是別的案例在驗的；這裡驗的是它問了多少。
		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
	})

	t.Run("the same day of a market that never shuts asks for a day of slots", func(t *testing.T) {
		fixture := buildFixture(t, vo.MarketCrypto)
		// 288 格加一 → 1445 根。
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "2330", sessionEnd, 1445).
			Return([]entities.KCandle{}, nil)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(
			t.Context(), requestOver(sessionEnd.Add(-24*time.Hour), sessionEnd))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
	})

	t.Run("a stretch a shutting market holds none of never reaches storage", func(t *testing.T) {
		fixture := buildFixture(t, vo.MarketTaiwanStock)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(
			t.Context(), requestOver(sessionEnd.Add(3*time.Hour), sessionEnd.Add(5*time.Hour)))

		assert.ErrorIs(t, err, domains.ErrObservationWindowHoldsNoTrading)
	})

	t.Run("a symbol nobody registered trades round the clock, as it always did", func(t *testing.T) {
		controller := gomock.NewController(t)
		kCandleRepository := mocks.NewMockIKCandleRepository(controller)
		tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
		tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").
			Return(entities.TradingSymbol{}, false, nil)
		clockProxy := mocks.NewMockIClockProxy(controller)
		clockProxy.EXPECT().Now().Return(askedAt).AnyTimes()
		// 沒登錄的代號落到永不收盤的市場，所以整段二十四小時都算數：288 格加一。
		kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "2330", sessionEnd, 1445).
			Return([]entities.KCandle{}, nil)

		indicatorCalculationService := service.NewIndicatorCalculationService(
			kCandleRepository, mocks.NewMockIKCandleSummaryDayRepository(controller), tradingSymbolRepository,
			mocks.NewMockIIndicatorScriptProxy(controller), clockProxy,
			marketCatalog(), maxCandleCount)

		_, err := indicatorCalculationService.CalculateIndicator(
			t.Context(), requestOver(sessionEnd.Add(-24*time.Hour), sessionEnd))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
	})

	t.Run("storage refusing to say which market it is stops the calculation", func(t *testing.T) {
		controller := gomock.NewController(t)
		tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
		tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").
			Return(entities.TradingSymbol{}, false, errors.New("資料庫讀不到"))
		clockProxy := mocks.NewMockIClockProxy(controller)
		clockProxy.EXPECT().Now().Return(askedAt).AnyTimes()

		indicatorCalculationService := service.NewIndicatorCalculationService(
			mocks.NewMockIKCandleRepository(controller), mocks.NewMockIKCandleSummaryDayRepository(controller), tradingSymbolRepository,
			mocks.NewMockIIndicatorScriptProxy(controller), clockProxy,
			marketCatalog(), maxCandleCount)

		_, err := indicatorCalculationService.CalculateIndicator(
			t.Context(), requestOver(sessionStart, sessionEnd))

		assert.ErrorContains(t, err, "資料庫讀不到")
	})
}

func dailyCandle(month time.Month, day int, hour int, close string) entities.KCandle {
	return entities.KCandle{
		Symbol: "BTCUSDT", OpenTime: time.Date(2026, month, day, hour, 0, 0, 0, time.UTC),
		Open: decimal.RequireFromString(close), High: decimal.RequireFromString(close),
		Low: decimal.RequireFromString(close), Close: decimal.RequireFromString(close), Volume: decimal.RequireFromString("1"),
	}
}

func dailyRequestEndingNow(dayCount int) dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol: "BTCUSDT", AggregationInterval: "1d",
		StartTime: calculationNow.Truncate(24 * time.Hour).Add(-time.Duration(dayCount) * 24 * time.Hour),
		EndTime:   calculationNow, Script: "the script",
	}
}

func (fixture calculationUnderTest) candlesHandedToTheScript(
	t *testing.T, requestDto dto.IndicatorCalculationRequestDto,
) ([]vo.KCandleVo, dto.IndicatorCalculationResultDto) {
	t.Helper()
	handedCandles := []vo.KCandleVo{}
	fixture.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, _ domains.IndicatorResultTypeDomain,
			kCandleVos []vo.KCandleVo, _ domains.StrategyScriptParametersDomain,
		) (map[string]vo.IndicatorValueVo, error) {
			handedCandles = kCandleVos
			return map[string]vo.IndicatorValueVo{}, nil
		})

	resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(), requestDto)
	require.NoError(t, err)

	return handedCandles, resultDto
}

func TestCalculateIndicatorTakesFinishedDaysFromTheirSummaries(t *testing.T) {
	fixture := newCalculationUnderTest(t)
	theTwoDays := []entities.KCandle{
		dailyCandle(8, 27, 0, "100"), dailyCandle(8, 27, 23, "110"),
		dailyCandle(8, 28, 0, "120"), dailyCandle(8, 28, 12, "90"), dailyCandle(8, 28, 23, "130"),
	}
	fixture.kCandleSummaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), "BTCUSDT", vo.AggregationIntervalOneDay,
			time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)).
		DoAndReturn(func(context.Context, string, vo.AggregationIntervalVo, time.Time, time.Time) ([]entities.KCandleSummaryDay, error) {
			summaryDays := []entities.KCandleSummaryDay{}
			for _, day := range []time.Time{time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)} {
				summaryDay := domains.NewKCandleSummaryDayDomain("BTCUSDT", day, 1, theTwoDays).ToEntity()
				dailySummaries := []entities.KCandleSummary{}
				for _, summary := range summaryDay.Summaries {
					if summary.Interval == string(vo.AggregationIntervalOneDay) && !summary.OpenTime.Before(day) &&
						summary.OpenTime.Before(day.Add(24*time.Hour)) {
						dailySummaries = append(dailySummaries, summary)
					}
				}
				summaryDay.Summaries = dailySummaries
				summaryDays = append(summaryDays, summaryDay)
			}
			return summaryDays, nil
		})

	kCandleVos, _ := fixture.candlesHandedToTheScript(t, dailyRequestEndingNow(2))

	require.Len(t, kCandleVos, 2)
	assert.Equal(t, time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC).Unix(), kCandleVos[0].OpenTimeUnixSeconds)
	assert.Equal(t, vo.KCandleVo{Symbol: "BTCUSDT", OpenTimeUnixSeconds: time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC).Unix(),
		Open: 120, High: 130, Low: 90, Close: 130, Volume: 3}, kCandleVos[1])
}

func TestCalculateIndicatorReachesPastStretchesWithNoMarketUntilEnoughBucketsAreFound(t *testing.T) {
	testCases := []struct {
		name              string
		held              []entities.KCandle
		summarizedDays    []time.Time
		dayCount          int
		expectedOpenTimes []time.Time
		expectedRequired  int
	}{
		{
			name: "a halt of more than two weeks is skipped",
			held: []entities.KCandle{
				dailyCandle(8, 28, 5, "100"), dailyCandle(8, 27, 5, "100"),
				dailyCandle(8, 10, 5, "100"), dailyCandle(8, 9, 5, "100"), dailyCandle(8, 8, 5, "100"),
			},
			dayCount: 4,
			expectedOpenTimes: []time.Time{
				time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
				time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC),
			},
			expectedRequired: 4,
		},
		{
			name: "days already summarized and days merged on the way are counted alike",
			held: []entities.KCandle{
				dailyCandle(8, 28, 5, "100"), dailyCandle(8, 20, 5, "100"), dailyCandle(8, 19, 5, "100"),
			},
			summarizedDays: []time.Time{time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)},
			dayCount:       3,
			expectedOpenTimes: []time.Time{
				time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
				time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC),
			},
			expectedRequired: 3,
		},
		{
			name:              "a history shorter than asked for answers with every day it has",
			held:              []entities.KCandle{dailyCandle(8, 28, 5, "100"), dailyCandle(8, 3, 13, "100")},
			dayCount:          4,
			expectedOpenTimes: []time.Time{time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)},
			expectedRequired:  4,
		},
		{
			name:              "today's running day is never read",
			held:              []entities.KCandle{dailyCandle(8, 29, 1, "100"), dailyCandle(8, 28, 5, "100")},
			dayCount:          1,
			expectedOpenTimes: []time.Time{time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)},
			expectedRequired:  1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newCalculationUnderTest(t)
			fixture.holdKCandles(testCase.held, testCase.summarizedDays...)

			kCandleVos, resultDto := fixture.candlesHandedToTheScript(t, dailyRequestEndingNow(testCase.dayCount))

			assert.Equal(t, testCase.expectedOpenTimes, bucketOpenTimes(kCandleVos))
			assert.Equal(t, testCase.expectedRequired, resultDto.RequiredCandleCount)
			assert.Equal(t, len(testCase.expectedOpenTimes), resultDto.UsedCandleCount)
		})
	}
}

func bucketOpenTimes(kCandleVos []vo.KCandleVo) []time.Time {
	openTimes := make([]time.Time, 0, len(kCandleVos))
	for _, kCandleVo := range kCandleVos {
		openTimes = append(openTimes, time.Unix(kCandleVo.OpenTimeUnixSeconds, 0).UTC())
	}

	return openTimes
}

func TestCalculateIndicatorStopsWhenAnEarlierCandleCannotBeLookedFor(t *testing.T) {
	storageDown := errors.New("storage will not answer")
	fixture := newCalculationUnderTest(t)
	fixture.kCandleSummaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
	fixture.kCandleRepository.EXPECT().
		FindInRange(gomock.Any(), gomock.Any(), 1).Return([]entities.KCandle{}, nil)
	fixture.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), 1).Return(nil, storageDown)

	_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(), dailyRequestEndingNow(2))

	require.ErrorIs(t, err, storageDown)
}

func TestCalculateIndicatorStopsWhenSummariesCannotBeRead(t *testing.T) {
	storageDown := errors.New("storage will not answer")
	fixture := newCalculationUnderTest(t)
	fixture.kCandleSummaryDayRepository.EXPECT().
		FindDaysInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, storageDown)

	_, err := fixture.indicatorCalculationService.CalculateIndicator(t.Context(), dailyRequestEndingNow(2))

	require.ErrorIs(t, err, storageDown)
}

func TestCalculateIndicatorSummarizesTheMissingDaysItReads(t *testing.T) {
	fixture := newCalculationUnderTest(t)
	summarizedDays := []time.Time{}
	fixture.kCandleSummaryDayRepository.EXPECT().SaveSummarized(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, summaryDay entities.KCandleSummaryDay) (bool, error) {
			summarizedDays = append(summarizedDays, summaryDay.Day)
			return true, nil
		}).Times(3)
	fixture.holdKCandles([]entities.KCandle{
		dailyCandle(8, 26, 5, "100"), dailyCandle(8, 27, 5, "110"), dailyCandle(8, 28, 5, "120"),
	})

	kCandleVos, _ := fixture.candlesHandedToTheScript(t, dailyRequestEndingNow(3))

	assert.Equal(t, []time.Time{
		time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC),
	}, summarizedDays)
	assert.Equal(t, []float64{100, 110, 120}, []float64{kCandleVos[0].Close, kCandleVos[1].Close, kCandleVos[2].Close})
}
