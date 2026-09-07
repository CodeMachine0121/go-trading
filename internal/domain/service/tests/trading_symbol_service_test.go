package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// registrationTime is the moment every registration in these tests is stamped with,
// so that "when was this registered" is a value the tests state rather than whatever
// the wall clock said while they ran.
var registrationTime = time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)

type tradingSymbolServiceUnderTest struct {
	tradingSymbolService    *service.TradingSymbolService
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	kCandleRepository       *mocks.MockIKCandleRepository
	symbolLookupProxy       *mocks.MockISymbolLookupProxy
}

func newTradingSymbolServiceUnderTest(t *testing.T) tradingSymbolServiceUnderTest {
	controller := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	symbolLookupProxy := mocks.NewMockISymbolLookupProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(registrationTime).AnyTimes()

	return tradingSymbolServiceUnderTest{
		tradingSymbolService: service.NewTradingSymbolService(
			tradingSymbolRepository, kCandleRepository, symbolLookupProxy, clockProxy, testMarketCatalog()),
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
		symbolLookupProxy:       symbolLookupProxy,
	}
}

// testMarketCatalog is the two markets these tests are written against: the
// round-the-clock one this system started with, and a Taiwan session that closes.
func testMarketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
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

func registered(symbols ...string) []entities.TradingSymbol {
	tradingSymbols := make([]entities.TradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		tradingSymbols = append(tradingSymbols, entities.TradingSymbol{Symbol: symbol})
	}

	return tradingSymbols
}

// newlyRegistered is the shape a market the system ships knowing about is written in:
// the market this system started with, and already watched — otherwise a fresh
// install would sit there fetching nothing.
func newlyRegistered(symbols ...string) []entities.TradingSymbol {
	tradingSymbols := make([]entities.TradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		tradingSymbols = append(tradingSymbols, entities.TradingSymbol{
			Symbol:       symbol,
			Market:       string(vo.MarketCrypto),
			IsWatched:    true,
			RegisteredAt: registrationTime,
		})
	}

	return tradingSymbols
}

func TestListTradingSymbols(t *testing.T) {
	testCases := []struct {
		name              string
		registeredSymbols []string
		heldSymbols       []string
		expectedSymbols   []string
	}{
		{
			name:              "registered markets appear even with no candles at all",
			registeredSymbols: []string{"BTCUSDT", "ETHUSDT"}, heldSymbols: []string{},
			expectedSymbols: []string{"BTCUSDT", "ETHUSDT"},
		},
		{
			name:              "a market nobody registered but that has a candle appears too",
			registeredSymbols: []string{"BTCUSDT"}, heldSymbols: []string{"XRPUSDT"},
			expectedSymbols: []string{"BTCUSDT", "XRPUSDT"},
		},
		{
			name:              "a market on both sides appears once",
			registeredSymbols: []string{"BTCUSDT"}, heldSymbols: []string{"BTCUSDT"},
			expectedSymbols: []string{"BTCUSDT"},
		},
		{
			name:              "the two sides are merged by name, not concatenated",
			registeredSymbols: []string{"SOLUSDT"}, heldSymbols: []string{"BTCUSDT"},
			expectedSymbols: []string{"BTCUSDT", "SOLUSDT"},
		},
		{
			// ETHUSDT is registered but holds nothing, while BTCUSDT still holds candles:
			// only a list that keeps registered-but-empty markets answers with both.
			name:              "a registered market stays listed after its candles are deleted",
			registeredSymbols: []string{"ETHUSDT"}, heldSymbols: []string{"BTCUSDT"},
			expectedSymbols: []string{"BTCUSDT", "ETHUSDT"},
		},
		{
			name:              "nothing on either side is an empty list",
			registeredSymbols: []string{}, heldSymbols: []string{},
			expectedSymbols: []string{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newTradingSymbolServiceUnderTest(t)
			fixture.tradingSymbolRepository.EXPECT().
				FindAll(gomock.Any()).Return(registered(testCase.registeredSymbols...), nil)
			fixture.kCandleRepository.EXPECT().
				FindDistinctSymbols(gomock.Any()).Return(testCase.heldSymbols, nil)

			tradingSymbolDtos, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

			assert.NoError(t, err)
			expectedDtos := make([]dto.TradingSymbolDto, 0, len(testCase.expectedSymbols))
			for _, expectedSymbol := range testCase.expectedSymbols {
				expectedDtos = append(expectedDtos, dto.TradingSymbolDto{Symbol: expectedSymbol})
			}
			assert.Equal(t, expectedDtos, tradingSymbolDtos)
			assert.NotNil(t, tradingSymbolDtos)
		})
	}

	t.Run("reports a failure reading the registered markets", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		storageFailure := errors.New("storage unreachable")
		fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return(nil, storageFailure)

		_, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

		assert.ErrorIs(t, err, storageFailure)
	})

	t.Run("reports a failure reading the markets that have candles", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		storageFailure := errors.New("storage unreachable")
		fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return(registered("BTCUSDT"), nil)
		fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return(nil, storageFailure)

		_, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

		assert.ErrorIs(t, err, storageFailure)
	})
}

func TestRegisterDefaultTradingSymbols(t *testing.T) {
	t.Run("registers both defaults on a database that has none", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return(registered(), nil)
		fixture.tradingSymbolRepository.EXPECT().
			RegisterAll(gomock.Any(), newlyRegistered("BTCUSDT", "ETHUSDT")).Return(nil)

		registeredNames, err := fixture.tradingSymbolService.RegisterDefaultTradingSymbols(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, []string{"BTCUSDT", "ETHUSDT"}, registeredNames)
	})

	t.Run("registers nothing and says so when both are already there", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().
			FindAll(gomock.Any()).Return(registered("BTCUSDT", "ETHUSDT"), nil)
		fixture.tradingSymbolRepository.EXPECT().RegisterAll(gomock.Any(), newlyRegistered()).Return(nil)

		registeredNames, err := fixture.tradingSymbolService.RegisterDefaultTradingSymbols(t.Context())

		assert.NoError(t, err)
		assert.Empty(t, registeredNames)
	})

	t.Run("registers only the one that is missing", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return(registered("BTCUSDT"), nil)
		fixture.tradingSymbolRepository.EXPECT().RegisterAll(gomock.Any(), newlyRegistered("ETHUSDT")).Return(nil)

		registeredNames, err := fixture.tradingSymbolService.RegisterDefaultTradingSymbols(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, []string{"ETHUSDT"}, registeredNames)
	})

	t.Run("leaves markets nobody asked about alone", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().
			FindAll(gomock.Any()).Return(registered("BTCUSDT", "ETHUSDT", "XRPUSDT"), nil)
		fixture.tradingSymbolRepository.EXPECT().RegisterAll(gomock.Any(), newlyRegistered()).Return(nil)

		registeredNames, err := fixture.tradingSymbolService.RegisterDefaultTradingSymbols(t.Context())

		assert.NoError(t, err)
		assert.Empty(t, registeredNames)
	})

	t.Run("never writes when it cannot read what is already registered", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		storageFailure := errors.New("storage unreachable")
		fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return(nil, storageFailure)

		_, err := fixture.tradingSymbolService.RegisterDefaultTradingSymbols(t.Context())

		assert.ErrorIs(t, err, storageFailure)
	})

	t.Run("reports a failure while registering", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		storageFailure := errors.New("storage unreachable")
		fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return(registered(), nil)
		fixture.tradingSymbolRepository.EXPECT().RegisterAll(gomock.Any(), gomock.Any()).Return(storageFailure)

		_, err := fixture.tradingSymbolService.RegisterDefaultTradingSymbols(t.Context())

		assert.ErrorIs(t, err, storageFailure)
	})
}

func TestAddToWatchlist(t *testing.T) {
	t.Run("checks with the market, then starts watching", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.symbolLookupProxy.EXPECT().
			SymbolExists(gomock.Any(), vo.MarketTaiwanStock, "2330").Return(true, nil)
		fixture.tradingSymbolRepository.EXPECT().
			FindBySymbol(gomock.Any(), "2330").Return(entities.TradingSymbol{}, false, nil)
		fixture.tradingSymbolRepository.EXPECT().Save(gomock.Any(), entities.TradingSymbol{
			Symbol:       "2330",
			Market:       "taiwanStock",
			IsWatched:    true,
			RegisteredAt: registrationTime,
		}).Return(nil)

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "2330", Market: "taiwanStock"})

		assert.NoError(t, addError)
	})

	t.Run("refuses a code the market has never heard of, and writes nothing", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.symbolLookupProxy.EXPECT().
			SymbolExists(gomock.Any(), vo.MarketTaiwanStock, "9999").Return(false, nil)

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "9999", Market: "taiwanStock"})

		assert.ErrorIs(t, addError, domains.ErrTradingSymbolNotInMarket)
	})

	t.Run("refuses when the market cannot be asked, and writes nothing", func(t *testing.T) {
		// Told apart from the case above on purpose: nothing is wrong with this
		// request, so the advice is to try again rather than to fix what was typed.
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.symbolLookupProxy.EXPECT().
			SymbolExists(gomock.Any(), vo.MarketTaiwanStock, "2330").
			Return(false, errors.New("market source unreachable"))

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "2330", Market: "taiwanStock"})

		assert.ErrorIs(t, addError, domains.ErrMarketDataSourceUnavailable)
		assert.NotErrorIs(t, addError, domains.ErrTradingSymbolNotInMarket)
	})

	t.Run("refuses a blank code without asking any market", func(t *testing.T) {
		// No expectation is set on the lookup, so reaching it would fail the test.
		// Paying a round trip to discover that nothing was typed is wasted either way.
		fixture := newTradingSymbolServiceUnderTest(t)

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "   ", Market: "taiwanStock"})

		assert.ErrorIs(t, addError, domains.ErrWatchlistEntryValidation)
	})

	t.Run("refuses a market nobody offers without asking any market", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "AAPL", Market: "nasdaq"})

		assert.ErrorIs(t, addError, domains.ErrWatchlistEntryValidation)
	})

	t.Run("adding one already watched leaves one entry", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		alreadyRegisteredAt := time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)
		fixture.symbolLookupProxy.EXPECT().
			SymbolExists(gomock.Any(), vo.MarketTaiwanStock, "2330").Return(true, nil)
		fixture.tradingSymbolRepository.EXPECT().
			FindBySymbol(gomock.Any(), "2330").
			Return(entities.TradingSymbol{
				Symbol: "2330", Market: "taiwanStock",
				IsWatched: true, RegisteredAt: alreadyRegisteredAt,
			}, true, nil)
		// Saved once, under the same name, so there is one entry rather than two.
		fixture.tradingSymbolRepository.EXPECT().Save(gomock.Any(), entities.TradingSymbol{
			Symbol:       "2330",
			Market:       "taiwanStock",
			IsWatched:    true,
			RegisteredAt: alreadyRegisteredAt,
		}).Return(nil)

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "2330", Market: "taiwanStock"})

		assert.NoError(t, addError)
	})

	t.Run("adding back one that was removed keeps its place in the queue", func(t *testing.T) {
		// Registration order decides who gets a market's live-follow places. Sending
		// somebody to the back of that queue for having briefly stopped watching them
		// would make removing a symbol quietly expensive.
		fixture := newTradingSymbolServiceUnderTest(t)
		originallyRegisteredAt := time.Date(2026, 8, 1, 1, 0, 0, 0, time.UTC)
		fixture.symbolLookupProxy.EXPECT().
			SymbolExists(gomock.Any(), vo.MarketTaiwanStock, "2330").Return(true, nil)
		fixture.tradingSymbolRepository.EXPECT().
			FindBySymbol(gomock.Any(), "2330").
			Return(entities.TradingSymbol{
				Symbol: "2330", Market: "taiwanStock",
				IsWatched: false, RegisteredAt: originallyRegisteredAt,
			}, true, nil)
		fixture.tradingSymbolRepository.EXPECT().Save(gomock.Any(), entities.TradingSymbol{
			Symbol:       "2330",
			Market:       "taiwanStock",
			IsWatched:    true,
			RegisteredAt: originallyRegisteredAt,
		}).Return(nil)

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "2330", Market: "taiwanStock"})

		assert.NoError(t, addError)
	})

	t.Run("reports a failure reading what is already registered", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		storageFailure := errors.New("storage unreachable")
		fixture.symbolLookupProxy.EXPECT().
			SymbolExists(gomock.Any(), vo.MarketTaiwanStock, "2330").Return(true, nil)
		fixture.tradingSymbolRepository.EXPECT().
			FindBySymbol(gomock.Any(), "2330").Return(entities.TradingSymbol{}, false, storageFailure)

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "2330", Market: "taiwanStock"})

		assert.ErrorIs(t, addError, storageFailure)
	})
}

func TestRemoveFromWatchlist(t *testing.T) {
	t.Run("stops watching without removing anything else", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		registeredAt := time.Date(2026, 8, 1, 1, 0, 0, 0, time.UTC)
		fixture.tradingSymbolRepository.EXPECT().
			FindBySymbol(gomock.Any(), "2330").
			Return(entities.TradingSymbol{
				Symbol: "2330", Market: "taiwanStock",
				IsWatched: true, RegisteredAt: registeredAt,
			}, true, nil)
		// Still registered, still in the same place in the queue — only the watching
		// stops. Candles are not this method's to touch and it touches none.
		fixture.tradingSymbolRepository.EXPECT().Save(gomock.Any(), entities.TradingSymbol{
			Symbol:       "2330",
			Market:       "taiwanStock",
			IsWatched:    false,
			RegisteredAt: registeredAt,
		}).Return(nil)

		removeError := fixture.tradingSymbolService.RemoveFromWatchlist(t.Context(), "2330")

		assert.NoError(t, removeError)
	})

	t.Run("removing one nobody registered is not a failure", func(t *testing.T) {
		// What the caller asked for is already true.
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().
			FindBySymbol(gomock.Any(), "2330").Return(entities.TradingSymbol{}, false, nil)

		removeError := fixture.tradingSymbolService.RemoveFromWatchlist(t.Context(), "2330")

		assert.NoError(t, removeError)
	})

	t.Run("removing one that was not being watched writes nothing", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.tradingSymbolRepository.EXPECT().
			FindBySymbol(gomock.Any(), "2330").
			Return(entities.TradingSymbol{Symbol: "2330", IsWatched: false}, true, nil)

		removeError := fixture.tradingSymbolService.RemoveFromWatchlist(t.Context(), "2330")

		assert.NoError(t, removeError)
	})

	t.Run("refuses a blank code", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)

		removeError := fixture.tradingSymbolService.RemoveFromWatchlist(t.Context(), "   ")

		assert.ErrorIs(t, removeError, domains.ErrWatchlistEntryValidation)
	})

	t.Run("reports a failure reading what is registered", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		storageFailure := errors.New("storage unreachable")
		fixture.tradingSymbolRepository.EXPECT().
			FindBySymbol(gomock.Any(), "2330").Return(entities.TradingSymbol{}, false, storageFailure)

		removeError := fixture.tradingSymbolService.RemoveFromWatchlist(t.Context(), "2330")

		assert.ErrorIs(t, removeError, storageFailure)
	})
}
