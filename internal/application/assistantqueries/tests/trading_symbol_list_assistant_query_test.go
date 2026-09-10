package assistantqueries_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/application/assistantqueries"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type tradingSymbolListAssistantQueryUnderTest struct {
	assistantQuery          *assistantqueries.TradingSymbolListAssistantQuery
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

	// Nothing is watched unless a test says so, so a listing that also asks what
	// holds a market's live places finds none held.
	tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).
		Return([]entities.TradingSymbol{}, nil).AnyTimes()

	return tradingSymbolListAssistantQueryUnderTest{
		assistantQuery: assistantqueries.NewTradingSymbolListAssistantQuery(
			application.NewTradingSymbolApplication(
				service.NewTradingSymbolService(
					tradingSymbolRepository, kCandleRepository,
					mocks.NewMockISymbolLookupProxy(controller), tradingSymbolClockProxy(controller),
					tradingSymbolMarketCatalog()),
				service.NewKCandleIngestionService(
					kCandleRepository, tradingSymbolRepository,
					mocks.NewMockIMarketDataProxy(controller), tradingSymbolClockProxy(controller),
					tradingSymbolMarketCatalog(), 5, time.Hour))),
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
	}
}

func TestTradingSymbolListAssistantQueryHandsOverEveryMarketTheSystemKnows(t *testing.T) {
	fixture := newTradingSymbolListAssistantQueryUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).
		Return([]entities.TradingSymbol{{Symbol: "BTCUSDT"}, {Symbol: "ETHUSDT"}}, nil)
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)

	outcome, runError := fixture.assistantQuery.Run(t.Context(), assistantViewerID, "{}")

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

	outcome, runError := fixture.assistantQuery.Run(t.Context(), assistantViewerID, "{}")

	require.NoError(t, runError)
	assert.JSONEq(t, `{"symbols":[]}`, outcome)
}

func TestTradingSymbolListAssistantQueryReportsAFailureToRead(t *testing.T) {
	fixture := newTradingSymbolListAssistantQueryUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).
		Return(nil, errors.New("storage unavailable"))

	_, runError := fixture.assistantQuery.Run(t.Context(), assistantViewerID, "{}")

	require.Error(t, runError)
}

// tradingSymbolClockProxy stamps registrations with a moment the test states, rather
// than with whatever the wall clock said while it ran.
func tradingSymbolClockProxy(controller *gomock.Controller) *mocks.MockIClockProxy {
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)).AnyTimes()

	return clockProxy
}

// tradingSymbolMarketCatalog is the markets these tests are written against.
func tradingSymbolMarketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
	})
}
