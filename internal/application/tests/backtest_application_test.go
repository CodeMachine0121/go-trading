package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
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

// backtestNow sits well after every stretch replayed below, so nothing is refused
// merely for reaching into an hour that has not finished.
var backtestNow = time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)

var backtestStart = time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)

type backtestUnderTest struct {
	backtestApplication  *application.BacktestApplication
	kCandleRepository    *mocks.MockIKCandleRepository
	indicatorScriptProxy *mocks.MockIIndicatorScriptProxy
}

// newBacktestUnderTest wires the real domain service and real domain models, mocking
// only the outermost boundaries: storage and script execution.
func newBacktestUnderTest(t *testing.T) backtestUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(backtestNow).AnyTimes()

	return backtestUnderTest{
		backtestApplication: application.NewBacktestApplication(
			service.NewBacktestService(
				kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults)),
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
	}
}

func backtestRequestDto() dto.BacktestRequestDto {
	return dto.BacktestRequestDto{
		Symbol:              "BTCUSDT",
		AggregationInterval: "1h",
		StartTime:           backtestStart,
		EndTime:             backtestStart.Add(4 * time.Hour),
		Script:              "the script",
		InitialCapital:      decimal.NewFromInt(10000),
		PositionSizingMode:  "allIn",
	}
}

// storedHourlyCandle builds a stored candle that many hours into the stretch.
func storedHourlyCandle(hour int, closePrice string) entities.KCandle {
	return entities.KCandle{
		Symbol:   "BTCUSDT",
		OpenTime: backtestStart.Add(time.Duration(hour) * time.Hour),
		Open:     decimal.RequireFromString(closePrice),
		High:     decimal.RequireFromString(closePrice),
		Low:      decimal.RequireFromString(closePrice),
		Close:    decimal.RequireFromString(closePrice),
	}
}

// signalsSaying builds the per-candle results a script would have produced.
func signalsSaying(signalStrengths ...float64) []map[string]vo.IndicatorValueVo {
	perCandleIndicatorValues := make([]map[string]vo.IndicatorValueVo, 0, len(signalStrengths))
	for _, signalStrength := range signalStrengths {
		perCandleIndicatorValues = append(perCandleIndicatorValues,
			map[string]vo.IndicatorValueVo{
				domains.SignalIndicatorName: {Numbers: []float64{signalStrength}},
			})
	}

	return perCandleIndicatorValues
}

func TestRunBacktest(t *testing.T) {
	t.Run("reads the stretch asked for, stopping at the hour still running", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context, query domains.KCandleQueryDomain, limit int,
			) ([]entities.KCandle, error) {
				assert.Equal(t, "BTCUSDT", query.Symbol())
				assert.Equal(t, backtestStart, query.StartTime())
				assert.Equal(t, backtestStart.Add(4*time.Hour), query.EndTime())
				// Five hourly buckets plus one spare, twelve five-minute candles each.
				assert.Equal(t, 6*12, limit)
				return []entities.KCandle{
					storedHourlyCandle(0, "100"), storedHourlyCandle(1, "110"),
				}, nil
			})
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(signalsSaying(0, 0), nil)

		_, err := fixture.backtestApplication.RunBacktest(t.Context(), backtestRequestDto())

		assert.NoError(t, err)
	})

	t.Run("replays the script over the finished hours, oldest first", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{
				storedHourlyCandle(0, "100"),
				storedHourlyCandle(1, "110"),
				storedHourlyCandle(2, "120"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), "the script", gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context,
				_ string,
				resultType domains.IndicatorResultTypeDomain,
				kCandleVos []vo.KCandleVo,
				_ domains.StrategyParametersDomain,
			) ([]map[string]vo.IndicatorValueVo, error) {
				// A replayed script always produces one number per indicator: the signal.
				assert.Equal(t, vo.IndicatorResultTypeFloat, resultType.Value())
				require.Len(t, kCandleVos, 3)
				assert.Equal(t, backtestStart.Unix(), kCandleVos[0].OpenTimeUnixSeconds)
				assert.Equal(t, 100.0, kCandleVos[0].Close)
				assert.Equal(t, 120.0, kCandleVos[2].Close)
				return signalsSaying(0, 0, 0), nil
			})

		_, err := fixture.backtestApplication.RunBacktest(t.Context(), backtestRequestDto())

		assert.NoError(t, err)
	})

	t.Run("hands back the report card, the trades and the curve together", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{
				storedHourlyCandle(0, "100"),
				storedHourlyCandle(1, "110"),
				storedHourlyCandle(2, "120"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(signalsSaying(1, -1, 0), nil)

		result, err := fixture.backtestApplication.RunBacktest(t.Context(), backtestRequestDto())

		require.NoError(t, err)
		assert.Equal(t, "BTCUSDT", result.Symbol)
		assert.Equal(t, "1h", result.Interval)
		assert.Equal(t, backtestStart, result.StartTime)
		assert.Equal(t, backtestStart.Add(2*time.Hour), result.EndTime)
		assert.Equal(t, 3, result.UsedCandleCount)
		assert.Len(t, result.EquityCurve, 3)
		// Long 100 to 110 makes 1,000; the short it reversed into is still open at 120.
		require.Len(t, result.ClosedTrades, 1)
		assert.True(t, decimal.NewFromInt(1000).Equal(result.ClosedTrades[0].Profit),
			"profit was %s", result.ClosedTrades[0].Profit)
		assert.Equal(t, 2, result.Summary.PositionOpenCount)
	})

	t.Run("a strategy that never speaks reports no trades rather than a failure", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{
				storedHourlyCandle(0, "100"), storedHourlyCandle(1, "110"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]map[string]vo.IndicatorValueVo{
				{"ma": {Numbers: []float64{100}}}, {"ma": {Numbers: []float64{105}}},
			}, nil)

		result, err := fixture.backtestApplication.RunBacktest(t.Context(), backtestRequestDto())

		require.NoError(t, err)
		assert.Empty(t, result.ClosedTrades)
		assert.Equal(t, 0, result.Summary.PositionOpenCount)
		assert.Nil(t, result.Summary.WinRate)
		assert.Len(t, result.EquityCurve, 2)
	})

	t.Run("a stretch holding one finished hour is refused", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{storedHourlyCandle(0, "100")}, nil)

		_, err := fixture.backtestApplication.RunBacktest(t.Context(), backtestRequestDto())

		assert.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("a stretch holding nothing is refused", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil)

		_, err := fixture.backtestApplication.RunBacktest(t.Context(), backtestRequestDto())

		assert.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("a stretch that ends before it starts is refused without reading storage", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		requestDto := backtestRequestDto()
		requestDto.StartTime = backtestStart.Add(10 * time.Hour)
		requestDto.EndTime = backtestStart

		_, err := fixture.backtestApplication.RunBacktest(t.Context(), requestDto)

		assert.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("starting capital of zero is refused without reading storage", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		requestDto := backtestRequestDto()
		requestDto.InitialCapital = decimal.Zero

		_, err := fixture.backtestApplication.RunBacktest(t.Context(), requestDto)

		assert.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("a script that could not run brings the replay down", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{
				storedHourlyCandle(0, "100"), storedHourlyCandle(1, "110"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, domains.ErrIndicatorScriptFailed)

		_, err := fixture.backtestApplication.RunBacktest(t.Context(), backtestRequestDto())

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	})

	t.Run("a knob nobody declared is reported by name", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{
				storedHourlyCandle(0, "100"), storedHourlyCandle(1, "110"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, domains.UndeclaredParameter("period"))

		_, err := fixture.backtestApplication.RunBacktest(t.Context(), backtestRequestDto())

		parameterName, isUndeclared := domains.UndeclaredParameterName(err)
		assert.True(t, isUndeclared)
		assert.Equal(t, "period", parameterName)
	})

	t.Run("storage refusing to answer is passed on as it came", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		storageError := errors.New("storage is down")
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, storageError)

		_, err := fixture.backtestApplication.RunBacktest(t.Context(), backtestRequestDto())

		assert.ErrorIs(t, err, storageError)
	})
}
