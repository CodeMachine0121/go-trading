package application_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// indicatorNow is the moment every calculation below is asked at. It sits on a
// five-minute edge, so the candle at 09:10 belongs to a bucket that has finished.
var indicatorNow = at(9, 15)

// indicatorCutoff is where a read stops at five-minute coarseness: the start of the
// five minutes still running.
var indicatorCutoff = indicatorNow

type indicatorUnderTest struct {
	indicatorCalculationApplication *application.IndicatorCalculationApplication
	kCandleRepository               *mocks.MockIKCandleRepository
	indicatorScriptProxy            *mocks.MockIIndicatorScriptProxy
}

// newIndicatorUnderTest wires the real domain service and real domain models,
// mocking only the outermost boundaries: storage and script execution.
func newIndicatorUnderTest(t *testing.T) indicatorUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(indicatorNow).AnyTimes()

	return indicatorUnderTest{
		indicatorCalculationApplication: application.NewIndicatorCalculationApplication(
			service.NewIndicatorCalculationService(
				kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults)),
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
	}
}

func indicatorRequest(candleCount int) dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol: "BTCUSDT", CandleCount: candleCount, Script: "the script",
	}
}

func TestIndicatorCalculationApplication(t *testing.T) {
	t.Run("hands back the indicator values and the candles used", func(t *testing.T) {
		fixture := newIndicatorUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorCutoff, 3).
			Return([]entities.KCandle{kCandleAt(at(9, 10), "100"), kCandleAt(at(9, 5), "100"), kCandleAt(at(9, 0), "100")}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), "the script", gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		resultDto, err := fixture.indicatorCalculationApplication.CalculateIndicator(t.Context(), indicatorRequest(2))

		assert.NoError(t, err)
		assert.Equal(t, "BTCUSDT", resultDto.Symbol)
		assert.Equal(t, 2, resultDto.UsedCandleCount)
		assert.Equal(t, []float64{110}, resultDto.Values["ma"].Numbers)
		assert.Equal(t, "5m", resultDto.Interval)
		assert.Equal(t, []time.Time{at(9, 5), at(9, 10)}, resultDto.OpenTimes)
	})

	t.Run("reads at the coarseness asked for, up to the stretch that has finished", func(t *testing.T) {
		// One hour is twelve five-minute candles, so two buckets plus the spare is a
		// read of 36; and at 09:15 the hour that began at 09:00 has not finished, so
		// reading stops there rather than at the five-minute edge.
		fixture := newIndicatorUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", at(9, 0), 36).
			Return([]entities.KCandle{
				kCandleAt(at(8, 5), "100"), kCandleAt(at(8, 0), "100"),
				kCandleAt(at(7, 0), "100"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		requestDto := indicatorRequest(2)
		requestDto.AggregationInterval = "1h"

		resultDto, err := fixture.indicatorCalculationApplication.CalculateIndicator(t.Context(), requestDto)

		assert.NoError(t, err)
		assert.Equal(t, "1h", resultDto.Interval)
		assert.Equal(t, 2, resultDto.UsedCandleCount)
		assert.Equal(t, []time.Time{at(7, 0), at(8, 0)}, resultDto.OpenTimes,
			"兩根一小時的彙總 K 線，起始時間是那兩格的起點")
	})

	t.Run("refuses a request whose candle count is not usable", func(t *testing.T) {
		fixture := newIndicatorUnderTest(t)

		_, err := fixture.indicatorCalculationApplication.CalculateIndicator(t.Context(), indicatorRequest(0))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "計算根數必須大於零")
	})

	t.Run("refuses when too few finished buckets are there", func(t *testing.T) {
		fixture := newIndicatorUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorCutoff, 4).
			Return([]entities.KCandle{kCandleAt(at(9, 0), "100")}, nil)

		_, err := fixture.indicatorCalculationApplication.CalculateIndicator(t.Context(), indicatorRequest(3))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "湊得出 1 根，但要求 3 根")
	})

	t.Run("passes a script failure through untouched", func(t *testing.T) {
		fixture := newIndicatorUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorCutoff, 2).
			Return([]entities.KCandle{kCandleAt(at(9, 5), "100"), kCandleAt(at(9, 0), "100")}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("%w: 算式無法解讀", domains.ErrIndicatorScriptFailed))

		resultDto, err := fixture.indicatorCalculationApplication.CalculateIndicator(t.Context(), indicatorRequest(1))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Contains(t, err.Error(), "算式無法解讀")
		assert.Empty(t, resultDto.Values)
	})
}
