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
	"github.com/stretchr/testify/require"
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
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(indicatorNow).AnyTimes()

	return indicatorUnderTest{
		indicatorCalculationApplication: application.NewIndicatorCalculationApplication(
			service.NewIndicatorCalculationService(
				kCandleRepository, tradingSymbolRepository, indicatorScriptProxy, clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				queryMaxResults)),
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
	}
}

// indicatorRequest asks about a stretch holding that many one-minute slots. BTCUSDT
// trades round the clock, so a minute of the clock is a minute of market.
func indicatorRequest(candleCount int) dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol:    "BTCUSDT",
		StartTime: indicatorNow.Add(-time.Duration(candleCount) * time.Minute),
		Script:    "the script",
	}
}

func TestIndicatorCalculationApplication(t *testing.T) {
	t.Run("hands back the indicator values and the candles used", func(t *testing.T) {
		fixture := newIndicatorUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorCutoff, 3).
			Return([]entities.KCandle{kCandleAt(at(9, 10), "100"), kCandleAt(at(9, 5), "100"), kCandleAt(at(9, 0), "100")}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), "the script", gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		resultDto, err := fixture.indicatorCalculationApplication.CalculateIndicator(t.Context(), indicatorRequest(2))

		assert.NoError(t, err)
		assert.Equal(t, "BTCUSDT", resultDto.Symbol)
		assert.Equal(t, 2, resultDto.UsedCandleCount)
		assert.Equal(t, []float64{110}, resultDto.Values["ma"].Numbers)
		assert.Equal(t, "1m", resultDto.Interval)
		assert.Equal(t, []time.Time{at(9, 5), at(9, 10)}, resultDto.OpenTimes)
	})

	t.Run("reads at the coarseness asked for, up to the stretch that has finished", func(t *testing.T) {
		// One hour is sixty one-minute candles, so two buckets plus the spare is a
		// read of 180; and at 09:15 the hour that began at 09:00 has not finished, so
		// reading stops there rather than at the one-minute edge.
		fixture := newIndicatorUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", at(9, 0), 180).
			Return([]entities.KCandle{
				kCandleAt(at(8, 5), "100"), kCandleAt(at(8, 0), "100"),
				kCandleAt(at(7, 0), "100"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		requestDto := indicatorRequest(2)
		requestDto.AggregationInterval = "1h"
		// Two hours of market, so two hourly slots: the stretch says how many, and a
		// slot is an hour at this coarseness.
		requestDto.StartTime = indicatorNow.Add(-2 * time.Hour)

		resultDto, err := fixture.indicatorCalculationApplication.CalculateIndicator(t.Context(), requestDto)

		assert.NoError(t, err)
		assert.Equal(t, "1h", resultDto.Interval)
		assert.Equal(t, 2, resultDto.UsedCandleCount)
		assert.Equal(t, []time.Time{at(7, 0), at(8, 0)}, resultDto.OpenTimes,
			"兩根一小時的彙總 K 線，起始時間是那兩格的起點")
	})

	t.Run("refuses a request whose stretch of market has no length", func(t *testing.T) {
		fixture := newIndicatorUnderTest(t)

		_, err := fixture.indicatorCalculationApplication.CalculateIndicator(t.Context(), indicatorRequest(0))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "起點必須早於終點")
	})

	t.Run("answers over a short stretch instead of refusing it", func(t *testing.T) {
		// Three buckets asked for, one stored. The line comes back shorter rather than
		// not at all, and the pair of counts is what says so.
		fixture := newIndicatorUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorCutoff, 4).
			Return([]entities.KCandle{kCandleAt(at(9, 0), "100")}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{}, nil)

		resultDto, err := fixture.indicatorCalculationApplication.CalculateIndicator(
			t.Context(), indicatorRequest(3))

		require.NoError(t, err)
		assert.Equal(t, 3, resultDto.RequiredCandleCount)
		assert.Equal(t, 1, resultDto.UsedCandleCount)
	})

	t.Run("refuses a stretch too thin to yield one value", func(t *testing.T) {
		fixture := newIndicatorUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorCutoff, 4).
			Return(nil, nil)

		_, err := fixture.indicatorCalculationApplication.CalculateIndicator(
			t.Context(), indicatorRequest(3))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationCandleCoverageTooThin)
		availableCandleCount, minimumCandleCount, isTooThin :=
			domains.CandleCoverageShortfall(err)
		require.True(t, isTooThin)
		assert.Equal(t, 0, availableCandleCount)
		assert.Equal(t, 1, minimumCandleCount)
	})

	t.Run("passes a script failure through untouched", func(t *testing.T) {
		fixture := newIndicatorUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), "BTCUSDT", indicatorCutoff, 2).
			Return([]entities.KCandle{kCandleAt(at(9, 5), "100"), kCandleAt(at(9, 0), "100")}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("%w: 算式無法解讀", domains.ErrIndicatorScriptFailed))

		resultDto, err := fixture.indicatorCalculationApplication.CalculateIndicator(t.Context(), indicatorRequest(1))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Contains(t, err.Error(), "算式無法解讀")
		assert.Empty(t, resultDto.Values)
	})
}
