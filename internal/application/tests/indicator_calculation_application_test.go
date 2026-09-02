package application_test

import (
	"fmt"
	"testing"

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

	return indicatorUnderTest{
		indicatorCalculationApplication: application.NewIndicatorCalculationApplication(
			service.NewIndicatorCalculationService(kCandleRepository, indicatorScriptProxy, queryMaxResults)),
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
			FindLatest("BTCUSDT", 3).
			Return([]entities.KCandle{kCandleAt(at(9, 10), "100"), kCandleAt(at(9, 5), "100"), kCandleAt(at(9, 0), "100")}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute("the script", gomock.Any(), gomock.Any()).
			Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

		resultDto, err := fixture.indicatorCalculationApplication.CalculateIndicator(indicatorRequest(2))

		assert.NoError(t, err)
		assert.Equal(t, "BTCUSDT", resultDto.Symbol)
		assert.Equal(t, 2, resultDto.UsedCandleCount)
		assert.Equal(t, []float64{110}, resultDto.Values["ma"].Numbers)
	})

	t.Run("refuses a request whose candle count is not usable", func(t *testing.T) {
		fixture := newIndicatorUnderTest(t)

		_, err := fixture.indicatorCalculationApplication.CalculateIndicator(indicatorRequest(0))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "計算根數必須大於零")
	})

	t.Run("refuses when too few candles remain after dropping the newest", func(t *testing.T) {
		fixture := newIndicatorUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatest("BTCUSDT", 4).
			Return([]entities.KCandle{kCandleAt(at(9, 0), "100")}, nil)

		_, err := fixture.indicatorCalculationApplication.CalculateIndicator(indicatorRequest(3))

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "可用 0 根")
	})

	t.Run("passes a script failure through untouched", func(t *testing.T) {
		fixture := newIndicatorUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindLatest("BTCUSDT", 2).
			Return([]entities.KCandle{kCandleAt(at(9, 5), "100"), kCandleAt(at(9, 0), "100")}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("%w: 算式無法解讀", domains.ErrIndicatorScriptFailed))

		resultDto, err := fixture.indicatorCalculationApplication.CalculateIndicator(indicatorRequest(1))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Contains(t, err.Error(), "算式無法解讀")
		assert.Empty(t, resultDto.Values)
	})
}
