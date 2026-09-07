package application_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type tradingSymbolApplicationUnderTest struct {
	tradingSymbolApplication *application.TradingSymbolApplication
	tradingSymbolRepository  *mocks.MockITradingSymbolRepository
	kCandleRepository        *mocks.MockIKCandleRepository
}

// newTradingSymbolApplicationUnderTest wires the real domain service, mocking only
// the outermost boundary: storage.
func newTradingSymbolApplicationUnderTest(t *testing.T) tradingSymbolApplicationUnderTest {
	controller := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)

	return tradingSymbolApplicationUnderTest{
		tradingSymbolApplication: application.NewTradingSymbolApplication(
			service.NewTradingSymbolService(
				tradingSymbolRepository, kCandleRepository, tradingSymbolClockProxy(controller))),
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
	}
}

func TestTradingSymbolApplicationListTradingSymbols(t *testing.T) {
	t.Run("hands back the registered markets and the ones holding candles, merged", func(t *testing.T) {
		fixture := newTradingSymbolApplicationUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().
			FindAll(gomock.Any()).Return([]entities.TradingSymbol{{Symbol: "ETHUSDT"}}, nil)
		fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{"BTCUSDT"}, nil)

		tradingSymbolDtos, err := fixture.tradingSymbolApplication.ListTradingSymbols(t.Context())

		assert.NoError(t, err)
		assert.Equal(t,
			[]dto.TradingSymbolDto{{Symbol: "BTCUSDT"}, {Symbol: "ETHUSDT"}},
			tradingSymbolDtos)
	})
}

func TestTradingSymbolApplicationRegisterDefaults(t *testing.T) {
	t.Run("reports which markets this run newly registered", func(t *testing.T) {
		fixture := newTradingSymbolApplicationUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return([]entities.TradingSymbol{}, nil)
		fixture.tradingSymbolRepository.EXPECT().RegisterAll(gomock.Any(), gomock.Any()).Return(nil)

		registeredNames, err := fixture.tradingSymbolApplication.RegisterDefaultTradingSymbols(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, []string{"BTCUSDT", "ETHUSDT"}, registeredNames)
	})
}

// tradingSymbolClockProxy stamps registrations with a moment the test states, rather
// than with whatever the wall clock said while it ran.
func tradingSymbolClockProxy(controller *gomock.Controller) *mocks.MockIClockProxy {
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)).AnyTimes()

	return clockProxy
}
