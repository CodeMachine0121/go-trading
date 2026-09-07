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
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type tradingSymbolApplicationUnderTest struct {
	tradingSymbolApplication *application.TradingSymbolApplication
	tradingSymbolRepository  *mocks.MockITradingSymbolRepository
	kCandleRepository        *mocks.MockIKCandleRepository
	symbolLookupProxy        *mocks.MockISymbolLookupProxy
	marketDataProxy          *mocks.MockIMarketDataProxy
}

// newTradingSymbolApplicationUnderTest wires the real domain service, mocking only
// the outermost boundary: storage.
func newTradingSymbolApplicationUnderTest(t *testing.T) tradingSymbolApplicationUnderTest {
	controller := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)

	// Nothing is watched unless a test says so, so a listing that also asks what
	// holds a market's live places finds none held.
	tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).
		Return([]entities.TradingSymbol{}, nil).AnyTimes()

	symbolLookupProxy := mocks.NewMockISymbolLookupProxy(controller)
	marketDataProxy := mocks.NewMockIMarketDataProxy(controller)

	return tradingSymbolApplicationUnderTest{
		tradingSymbolApplication: application.NewTradingSymbolApplication(
			service.NewTradingSymbolService(
				tradingSymbolRepository, kCandleRepository,
				symbolLookupProxy, tradingSymbolClockProxy(controller),
				tradingSymbolMarketCatalog()),
			service.NewKCandleIngestionService(
				kCandleRepository, tradingSymbolRepository,
				marketDataProxy, tradingSymbolClockProxy(controller),
				tradingSymbolMarketCatalog(), 5, time.Hour)),
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
		symbolLookupProxy:       symbolLookupProxy,
		marketDataProxy:         marketDataProxy,
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
		// Both sides merged, ordered by name. What each one says about its own market
		// is pinned where those rules live, not restated here.
		assert.Equal(t,
			[]dto.TradingSymbolDto{
				{Symbol: "BTCUSDT", Market: "crypto", HasLiveUpdates: true, IsWithinTradingSession: true},
				{Symbol: "ETHUSDT", Market: "crypto", HasLiveUpdates: true, IsWithinTradingSession: true},
			},
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

// tradingSymbolMarketCatalog is the markets these tests are written against.
func tradingSymbolMarketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
		// A market that closes is here so that adding to the watchlist can be tested
		// as it behaves for the market that needs it: the one whose scheduled rounds
		// have nothing left to collect after the bell.
		vo.MarketTaiwanStock: {
			TradingSession: vo.TradingSessionVo{
				Location:   time.FixedZone("Asia/Taipei", 8*60*60),
				DailyStart: 9 * time.Hour,
				DailyEnd:   13*time.Hour + 30*time.Minute,
				Weekdays: []time.Weekday{
					time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
				},
			},
			SimultaneousFollowCeiling: 5,
		},
	})
}

func TestTradingSymbolApplicationAddToWatchlist(t *testing.T) {
	t.Run("catches the symbol up on the spot, without waiting for a round", func(t *testing.T) {
		// Adding is only half of what somebody adding a symbol wants. Without the
		// other half they are left looking at an empty chart until a round comes
		// round — and after a market's close, until the next start-up.
		fixture := newTradingSymbolApplicationUnderTest(t)
		fixture.symbolLookupProxy.EXPECT().
			LookUpSymbol(gomock.Any(), vo.MarketCrypto, "BTCUSDT").Return(vo.SymbolListingVo{IsListed: true}, nil)
		fixture.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
			Return(entities.TradingSymbol{}, false, nil)
		fixture.tradingSymbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
		fixture.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
			entities.TradingSymbol{
				Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
			}, true, nil)
		fixture.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
			Return([]entities.KCandle{}, nil)
		caughtUp := make(chan struct{}, 1)
		fixture.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ vo.KCandleFetchWindowVo) ([]vo.MarketKCandleVo, error) {
				caughtUp <- struct{}{}

				return []vo.MarketKCandleVo{}, nil
			})

		addError := fixture.tradingSymbolApplication.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto)})

		assert.NoError(t, addError)
		assert.Len(t, caughtUp, 1, "加進觀察清單之後應該立刻去補那一檔，而不是等下一輪")
	})

	t.Run("a catch-up that fails does not undo the add", func(t *testing.T) {
		// The symbol is on the watchlist — that is what was asked for and it is true.
		// The ordinary rounds will reach it anyway, so refusing here would report a
		// failure about something that fixes itself, and lose something that worked.
		fixture := newTradingSymbolApplicationUnderTest(t)
		fixture.symbolLookupProxy.EXPECT().
			LookUpSymbol(gomock.Any(), vo.MarketCrypto, "BTCUSDT").Return(vo.SymbolListingVo{IsListed: true}, nil)
		fixture.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
			Return(entities.TradingSymbol{}, false, nil)
		fixture.tradingSymbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
		fixture.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
			Return(entities.TradingSymbol{}, false, errors.New("storage unreachable"))

		addError := fixture.tradingSymbolApplication.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto)})

		assert.NoError(t, addError)
	})

	t.Run("nothing is caught up when the add itself was refused", func(t *testing.T) {
		fixture := newTradingSymbolApplicationUnderTest(t)
		fixture.symbolLookupProxy.EXPECT().
			LookUpSymbol(gomock.Any(), vo.MarketTaiwanStock, "9999").Return(vo.SymbolListingVo{}, nil)

		addError := fixture.tradingSymbolApplication.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "9999", Market: string(vo.MarketTaiwanStock)})

		assert.ErrorIs(t, addError, domains.ErrTradingSymbolNotInMarket)
	})
}
