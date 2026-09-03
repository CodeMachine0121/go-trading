package application_test

import (
	"testing"

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
			service.NewTradingSymbolService(tradingSymbolRepository, kCandleRepository)),
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
