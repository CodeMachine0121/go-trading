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
			Execute(gomock.Any(), gomock.Any()).
			Return(map[string]float64{"ma": 110}, nil)

		_, err := fixture.indicatorCalculationService.CalculateIndicator(calculationRequest("BTCUSDT", 3))

		assert.NoError(t, err)
	})

	t.Run("hands the script the requested candles oldest first, without the newest", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute("the script", gomock.Any()).
			DoAndReturn(func(script string, kCandleVos []vo.KCandleVo) (map[string]float64, error) {
				assert.Len(t, kCandleVos, 3)
				assert.Equal(t, storedCandleAt(0).OpenTime.Unix(), kCandleVos[0].OpenTimeUnixSeconds)
				assert.Equal(t, storedCandleAt(5).OpenTime.Unix(), kCandleVos[1].OpenTimeUnixSeconds)
				assert.Equal(t, storedCandleAt(10).OpenTime.Unix(), kCandleVos[2].OpenTimeUnixSeconds)
				return map[string]float64{"ma": 110}, nil
			})

		_, err := fixture.indicatorCalculationService.CalculateIndicator(calculationRequest("BTCUSDT", 3))

		assert.NoError(t, err)
	})

	t.Run("reports the indicator values and how many candles were used", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 4).Return(newestFirst(15, 10, 5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any()).
			Return(map[string]float64{"high": 120, "low": 100}, nil)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequest("BTCUSDT", 3))

		assert.NoError(t, err)
		assert.Equal(t, "BTCUSDT", resultDto.Symbol)
		assert.Equal(t, 3, resultDto.UsedCandleCount)
		assert.Equal(t, 120.0, resultDto.Values["high"])
		assert.Equal(t, 100.0, resultDto.Values["low"])
	})

	t.Run("treats an empty set of indicator values as a success", func(t *testing.T) {
		fixture := newCalculationUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindLatest("BTCUSDT", 2).Return(newestFirst(5, 0), nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any()).
			Return(map[string]float64{}, nil)

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
			Execute(gomock.Any(), gomock.Any()).
			Return(nil, domains.ErrIndicatorScriptFailed)

		resultDto, err := fixture.indicatorCalculationService.CalculateIndicator(
			calculationRequest("BTCUSDT", 3))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Empty(t, resultDto.Values)
		assert.Equal(t, 0, resultDto.UsedCandleCount)
	})
}
