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

// newTradingSymbolListAssistantQueryUnderTest mocks only storage, so the assistant gets what a person's own request would.
func newTradingSymbolListAssistantQueryUnderTest(t *testing.T) tradingSymbolListAssistantQueryUnderTest {
	controller := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)

	// Nothing is watched unless a test says so, so no market's live places are held.
	tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).
		Return([]entities.TradingSymbol{}, nil).AnyTimes()

	return tradingSymbolListAssistantQueryUnderTest{
		assistantQuery: assistantqueries.NewTradingSymbolListAssistantQuery(
			application.NewTradingSymbolApplication(
				service.NewTradingSymbolService(
					tradingSymbolRepository, kCandleRepository,
					mocks.NewMockISymbolLookupProxy(controller), tradingSymbolClockProxy(controller),
					tradingSymbolMarketCatalog()), service.NewKCandleIngestionService(
					kCandleRepository, mocks.NewMockIKCandleHistorySyncRunRepository(controller), tradingSymbolRepository,
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
	// An empty system is an answer to relay, not a refusal.
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

// tradingSymbolClockProxy stamps registrations with a fixed moment instead of the wall clock.
func tradingSymbolClockProxy(controller *gomock.Controller) *mocks.MockIClockProxy {
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)).AnyTimes()

	return clockProxy
}

func tradingSymbolMarketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
	})
}
