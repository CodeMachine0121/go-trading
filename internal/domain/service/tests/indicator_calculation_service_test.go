package service_test

import (
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
	"go.uber.org/mock/gomock"
)

const maxCandleCount = 1000

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

type calculationUnderTest struct {
	indicatorCalculationService *service.IndicatorCalculationService
	kCandleRepository           *mocks.MockIKCandleRepository
	indicatorScriptProxy        *mocks.MockIIndicatorScriptProxy
}

func newCalculationUnderTest(t *testing.T) calculationUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)

	return calculationUnderTest{
		indicatorCalculationService: service.NewIndicatorCalculationService(
			kCandleRepository, indicatorScriptProxy, maxCandleCount),
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
	}
}

func TestCalculateIndicator(t *testing.T) {
	t.Run("asks storage for one candle more than requested", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatest("BTCUSDT", 4).
			Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(calculationRequest("BTCUSDT", 3))

		assert.NoError(t, err)
	})

	t.Run("hands the script the requested candles oldest first, without the newest", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute("the script", gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				script string,
				resultType domains.IndicatorResultTypeDomain,
				kCandleVos []vo.KCandleVo,
			) (map[string]vo.IndicatorValueVo, error) {
				assert.Len(t, kCandleVos, 3)
				assert.Equal(t, storedCandleAt(0).OpenTime.Unix(), kCandleVos[0].OpenTimeUnixSeconds)
				assert.Equal(t, storedCandleAt(5).OpenTime.Unix(), kCandleVos[1].OpenTimeUnixSeconds)
				assert.Equal(t, storedCandleAt(10).OpenTime.Unix(), kCandleVos[2].OpenTimeUnixSeconds)
				return map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil
			})

		_, err := fixture.indicatorCalculationService.CalculateIndicator(calculationRequest("BTCUSDT", 3))

		assert.NoError(t, err)
	})

	t.Run("reports the indicator values and how many candles were used", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{
				"high": {Numbers: []float64{120}}, "low": {Numbers: []float64{100}},
			}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequest("BTCUSDT", 3))

		assert.NoError(t, err)
		assert.Equal(t, "BTCUSDT", resultDto.Symbol)
		assert.Equal(t, 3, resultDto.UsedCandleCount)
		assert.Equal(t, []float64{120}, resultDto.Values["high"].Numbers)
		assert.Equal(t, []float64{100}, resultDto.Values["low"].Numbers)
	})

	t.Run("treats an empty set of indicator values as a success", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 2).Return(newestFirst(5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequest("BTCUSDT", 1))

		assert.NoError(t, err)
		assert.Empty(t, resultDto.Values)
		assert.Equal(t, 1, resultDto.UsedCandleCount)
	})

	t.Run("never reaches storage when the request breaks a rule", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(calculationRequest("BTCUSDT", 0))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "計算根數必須大於零")
	})

	t.Run("never reaches storage when too many candles are requested", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequest("BTCUSDT", maxCandleCount+1))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "超過單次可用的最大根數")
	})

	t.Run("never runs the script when too few candles remain", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 31).Return(newestFirst(0), nil)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(calculationRequest("BTCUSDT", 30))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "可用 0 根")
	})

	t.Run("reports a storage failure as neither a request nor a script problem", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		storageFailure := errors.New("storage unreachable")
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 4).Return(nil, storageFailure)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequest("BTCUSDT", 3))

		assert.ErrorIs(t, err, storageFailure)
		assert.NotErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.NotErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Empty(t, resultDto.Values)
	})

	t.Run("reports a script failure without any partial result", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, domains.ErrIndicatorScriptFailed)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequest("BTCUSDT", 3))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Empty(t, resultDto.Values)
		assert.Equal(t, 0, resultDto.UsedCandleCount)
	})
}

func TestCalculateIndicatorCarriesTheDeclaredResultType(t *testing.T) {
	t.Run("hands the script runner the kind that was declared", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				script string,
				resultType domains.IndicatorResultTypeDomain,
				kCandleVos []vo.KCandleVo,
			) (map[string]vo.IndicatorValueVo, error) {
				assert.Equal(t, vo.IndicatorResultTypeFloatList, resultType.Value())
				return map[string]vo.IndicatorValueVo{}, nil
			})

		_, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequestOf("BTCUSDT", 3, "floatList"))

		assert.NoError(t, err)
	})

	t.Run("reports the kind alongside the values", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{
				"red": {IsList: true, Booleans: []bool{true, false}},
			}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequestOf("BTCUSDT", 3, "boolList"))

		assert.NoError(t, err)
		assert.Equal(t, "boolList", resultDto.ResultType)
		assert.True(t, resultDto.Values["red"].IsList)
		assert.Equal(t, []bool{true, false}, resultDto.Values["red"].Booleans)
	})

	t.Run("reports one number per indicator when nothing was declared", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 2).Return(newestFirst(5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequest("BTCUSDT", 1))

		assert.NoError(t, err)
		assert.Equal(t, "float", resultDto.ResultType)
	})

	t.Run("never reaches storage when the declared kind is not on offer", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequestOf("BTCUSDT", 3, "string"))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "指標值種類只能是")
	})
}

func TestCalculateIndicatorKeepsEveryOtherRuleWhateverTheKindIs(t *testing.T) {
	t.Run("a candle count of zero is refused just the same", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequestOf("BTCUSDT", 0, "floatList"))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "計算根數必須大於零")
	})

	t.Run("too few usable candles is refused just the same, naming what is usable", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 31).Return(newestFirst(45, 40, 35), nil)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequestOf("BTCUSDT", 30, "bool"))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "可用 2 根")
	})

	t.Run("the candles handed to the script are chosen the same way", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				script string,
				resultType domains.IndicatorResultTypeDomain,
				kCandleVos []vo.KCandleVo,
			) (map[string]vo.IndicatorValueVo, error) {
				assert.Len(t, kCandleVos, 3)
				assert.Equal(t, storedCandleAt(0).OpenTime.Unix(), kCandleVos[0].OpenTimeUnixSeconds)
				assert.Equal(t, storedCandleAt(5).OpenTime.Unix(), kCandleVos[1].OpenTimeUnixSeconds)
				assert.Equal(t, storedCandleAt(10).OpenTime.Unix(), kCandleVos[2].OpenTimeUnixSeconds)
				return map[string]vo.IndicatorValueVo{}, nil
			})

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequestOf("BTCUSDT", 3, "floatList"))

		assert.NoError(t, err)
		assert.Equal(t, 3, resultDto.UsedCandleCount)
	})
}
