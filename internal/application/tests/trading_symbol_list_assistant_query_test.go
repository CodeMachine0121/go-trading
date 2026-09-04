package application_test

import (
	"errors"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type tradingSymbolListAssistantQueryUnderTest struct {
	assistantQuery          *application.TradingSymbolListAssistantQuery
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	kCandleRepository       *mocks.MockIKCandleRepository
}

// newTradingSymbolListAssistantQueryUnderTest wires the real domain service and real
// domain models, mocking only storage — so what the assistant is handed goes through
// every rule a person's own request goes through.
func newTradingSymbolListAssistantQueryUnderTest(t *testing.T) tradingSymbolListAssistantQueryUnderTest {
	controller := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)

	return tradingSymbolListAssistantQueryUnderTest{
		assistantQuery: application.NewTradingSymbolListAssistantQuery(
			application.NewTradingSymbolApplication(
				service.NewTradingSymbolService(tradingSymbolRepository, kCandleRepository))),
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
	}
}

func TestTradingSymbolListAssistantQueryHandsOverEveryMarketTheSystemKnows(t *testing.T) {
	fixture := newTradingSymbolListAssistantQueryUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).
		Return([]entities.TradingSymbol{{Symbol: "BTCUSDT"}, {Symbol: "ETHUSDT"}}, nil)
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)

	outcome, runError := fixture.assistantQuery.Run(t.Context(), "{}")

	require.NoError(t, runError)
	assert.JSONEq(t, `{"symbols":["BTCUSDT","ETHUSDT"]}`, outcome)
}

func TestTradingSymbolListAssistantQueryAnswersKnowingNoneAsAnEmptyList(t *testing.T) {
	// A freshly built system genuinely knows of none. That is an answer the assistant
	// can relay, not a refusal it has to work around.
	fixture := newTradingSymbolListAssistantQueryUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).
		Return([]entities.TradingSymbol{}, nil)
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)

	outcome, runError := fixture.assistantQuery.Run(t.Context(), "{}")

	require.NoError(t, runError)
	assert.JSONEq(t, `{"symbols":[]}`, outcome)
}

func TestTradingSymbolListAssistantQueryReportsAFailureToRead(t *testing.T) {
	fixture := newTradingSymbolListAssistantQueryUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).
		Return(nil, errors.New("storage unavailable"))

	_, runError := fixture.assistantQuery.Run(t.Context(), "{}")

	require.Error(t, runError)
}
