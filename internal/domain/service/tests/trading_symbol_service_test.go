package service_test

import (
	"context"
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

// registrationTime is a fixed stamp so registration times are stated rather than read from the wall clock.
var registrationTime = time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)

type tradingSymbolServiceUnderTest struct {
	tradingSymbolService    *service.TradingSymbolService
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	kCandleRepository       *mocks.MockIKCandleRepository
	symbolLookupProxy       *mocks.MockISymbolLookupProxy
}

// closedMarketTime is a Taipei evening: Taiwan is shut, the round-the-clock market is not.
var closedMarketTime = time.Date(2026, 9, 7, 13, 0, 0, 0, time.UTC)

func newTradingSymbolServiceUnderTest(t *testing.T) tradingSymbolServiceUnderTest {
	return newTradingSymbolServiceUnderTestAt(t, registrationTime)
}

func newTradingSymbolServiceUnderTestAt(
	t *testing.T, currentTime time.Time,
) tradingSymbolServiceUnderTest {
	controller := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	symbolLookupProxy := mocks.NewMockISymbolLookupProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()

	return tradingSymbolServiceUnderTest{
		tradingSymbolService: service.NewTradingSymbolService(
			tradingSymbolRepository, kCandleRepository, symbolLookupProxy, clockProxy, testMarketCatalog()),
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
		symbolLookupProxy:       symbolLookupProxy,
	}
}

// testMarketCatalog holds the round-the-clock market and a Taiwan session that closes.
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
			FollowsFixedRoster:         true,
			SimultaneousChannelCeiling: 1,
			SymbolsPerLiveChannel:      5,
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

// newlyRegistered is how default symbols are written: the crypto market, already watched.
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
			// ETHUSDT is registered but empty while BTCUSDT holds candles; only keeping registered-but-empty markets lists both.
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
			// Nothing is watched, so no live places are held.
			fixture.tradingSymbolRepository.EXPECT().
				FindWatched(gomock.Any()).Return([]entities.TradingSymbol{}, nil)

			tradingSymbolDtos, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

			assert.NoError(t, err)
			// These cases pin only which markets appear and their order; per-symbol fields are pinned separately.
			assert.Equal(t, testCase.expectedSymbols, namesOfListed(tradingSymbolDtos))
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
			LookUpSymbol(gomock.Any(), vo.MarketTaiwanStock, "2330").Return(vo.SymbolListingVo{IsListed: true}, nil)
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

	t.Run("stores what the venue calls it, alongside the code", func(t *testing.T) {
		// Bare four-digit codes are unreadable at a glance.
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.symbolLookupProxy.EXPECT().
			LookUpSymbol(gomock.Any(), vo.MarketTaiwanStock, "2330").
			Return(vo.SymbolListingVo{IsListed: true, DisplayName: "台積電"}, nil)
		fixture.tradingSymbolRepository.EXPECT().
			FindBySymbol(gomock.Any(), "2330").Return(entities.TradingSymbol{}, false, nil)
		fixture.tradingSymbolRepository.EXPECT().Save(gomock.Any(), entities.TradingSymbol{
			Symbol:       "2330",
			Market:       "taiwanStock",
			DisplayName:  "台積電",
			IsWatched:    true,
			RegisteredAt: registrationTime,
		}).Return(nil)

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "2330", Market: "taiwanStock"})

		assert.NoError(t, addError)
	})

	t.Run("writes the venue's name every time, so adding back is how a rename lands", func(t *testing.T) {
		// Adding back is the one moment the venue is asked again, so its current name wins.
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.symbolLookupProxy.EXPECT().
			LookUpSymbol(gomock.Any(), vo.MarketTaiwanStock, "2330").
			Return(vo.SymbolListingVo{IsListed: true, DisplayName: "台積電控股"}, nil)
		fixture.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(
			entities.TradingSymbol{
				Symbol: "2330", Market: "taiwanStock", DisplayName: "台積電",
				RegisteredAt: registrationTime,
			}, true, nil)
		fixture.tradingSymbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, saved entities.TradingSymbol) error {
				assert.Equal(t, "台積電控股", saved.DisplayName)

				return nil
			})

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "2330", Market: "taiwanStock"})

		assert.NoError(t, addError)
	})

	t.Run("a market that names nothing leaves the name empty rather than repeating the code", func(t *testing.T) {
		// A crypto pair is already its own name, so copying the code would show it twice.
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.symbolLookupProxy.EXPECT().
			LookUpSymbol(gomock.Any(), vo.MarketCrypto, "BTCUSDT").
			Return(vo.SymbolListingVo{IsListed: true}, nil)
		fixture.tradingSymbolRepository.EXPECT().
			FindBySymbol(gomock.Any(), "BTCUSDT").Return(entities.TradingSymbol{}, false, nil)
		fixture.tradingSymbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, saved entities.TradingSymbol) error {
				assert.Empty(t, saved.DisplayName)

				return nil
			})

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "BTCUSDT", Market: "crypto"})

		assert.NoError(t, addError)
	})

	t.Run("refuses a code the market has never heard of, and writes nothing", func(t *testing.T) {
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.symbolLookupProxy.EXPECT().
			LookUpSymbol(gomock.Any(), vo.MarketTaiwanStock, "9999").Return(vo.SymbolListingVo{}, nil)

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "9999", Market: "taiwanStock"})

		assert.ErrorIs(t, addError, domains.ErrTradingSymbolNotInMarket)
	})

	t.Run("refuses when the market cannot be asked, and writes nothing", func(t *testing.T) {
		// Distinct from the case above: the request is fine, so the advice is to retry rather than fix the input.
		fixture := newTradingSymbolServiceUnderTest(t)
		fixture.symbolLookupProxy.EXPECT().
			LookUpSymbol(gomock.Any(), vo.MarketTaiwanStock, "2330").
			Return(vo.SymbolListingVo{}, errors.New("market source unreachable"))

		addError := fixture.tradingSymbolService.AddToWatchlist(
			t.Context(), dto.WatchlistEntryDto{Symbol: "2330", Market: "taiwanStock"})

		assert.ErrorIs(t, addError, domains.ErrMarketDataSourceUnavailable)
		assert.NotErrorIs(t, addError, domains.ErrTradingSymbolNotInMarket)
	})

	t.Run("refuses a blank code without asking any market", func(t *testing.T) {
		// No lookup expectation is set, so reaching it would fail the test.
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
			LookUpSymbol(gomock.Any(), vo.MarketTaiwanStock, "2330").Return(vo.SymbolListingVo{IsListed: true}, nil)
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
		// Registration order decides live-follow places, so briefly unwatching must not send a symbol to the back of the queue.
		fixture := newTradingSymbolServiceUnderTest(t)
		originallyRegisteredAt := time.Date(2026, 8, 1, 1, 0, 0, 0, time.UTC)
		fixture.symbolLookupProxy.EXPECT().
			LookUpSymbol(gomock.Any(), vo.MarketTaiwanStock, "2330").Return(vo.SymbolListingVo{IsListed: true}, nil)
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
			LookUpSymbol(gomock.Any(), vo.MarketTaiwanStock, "2330").Return(vo.SymbolListingVo{IsListed: true}, nil)
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
		// Still registered with the same queue place; only watching stops and no candles are touched.
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

// namesOfListed is which markets a listing named, in the order it named them.
func namesOfListed(tradingSymbolDtos []dto.TradingSymbolDto) []string {
	names := make([]string, 0, len(tradingSymbolDtos))
	for _, tradingSymbolDto := range tradingSymbolDtos {
		names = append(names, tradingSymbolDto.Symbol)
	}

	return names
}

func TestEveryListedSymbolSaysWhichMarketItBelongsTo(t *testing.T) {
	fixture := newTradingSymbolServiceUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return([]entities.TradingSymbol{
		{Symbol: "2330", Market: "taiwanStock", IsWatched: true, RegisteredAt: registrationTime},
		{Symbol: "BTCUSDT", Market: "crypto", IsWatched: true, RegisteredAt: registrationTime},
		// Registered before markets were ever recorded, so it names none.
		{Symbol: "XRPUSDT"},
	}, nil)
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)
	fixture.tradingSymbolRepository.EXPECT().
		FindWatched(gomock.Any()).Return([]entities.TradingSymbol{}, nil)

	tradingSymbolDtos, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "taiwanStock", listed(t, tradingSymbolDtos, "2330").Market)
	assert.Equal(t, "crypto", listed(t, tradingSymbolDtos, "BTCUSDT").Market)
	// Reading an old row as the original market keeps it behaving as before.
	assert.Equal(t, "crypto", listed(t, tradingSymbolDtos, "XRPUSDT").Market)
}

func TestEveryListedSymbolSaysWhetherItsMarketIsTradingRightNow(t *testing.T) {
	// The console cannot tell a holiday from an outage, so the service says whether the market is trading.
	fixture := newTradingSymbolServiceUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return([]entities.TradingSymbol{
		{Symbol: "2330", Market: "taiwanStock"},
		{Symbol: "BTCUSDT", Market: "crypto"},
	}, nil)
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)
	fixture.tradingSymbolRepository.EXPECT().
		FindWatched(gomock.Any()).Return([]entities.TradingSymbol{}, nil)

	tradingSymbolDtos, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

	assert.NoError(t, err)
	// registrationTime is 01:00 universal, which is 09:00 in Taipei on a Monday.
	assert.True(t, listed(t, tradingSymbolDtos, "2330").IsWithinTradingSession)
	assert.True(t, listed(t, tradingSymbolDtos, "BTCUSDT").IsWithinTradingSession)
}

func TestAListingCarriesWhatEachVenueCallsItsSymbols(t *testing.T) {
	// The code keys everything, so both travel: the console shows the pair and sends back only the code.
	fixture := newTradingSymbolServiceUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return([]entities.TradingSymbol{
		{Symbol: "2330", Market: string(vo.MarketTaiwanStock), DisplayName: "台積電"},
		{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto)},
	}, nil)
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)
	fixture.tradingSymbolRepository.EXPECT().
		FindWatched(gomock.Any()).Return([]entities.TradingSymbol{}, nil)

	tradingSymbolDtos, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "台積電", listed(t, tradingSymbolDtos, "2330").DisplayName)
	// A venue that names nothing leaves it empty rather than repeating the code.
	assert.Empty(t, listed(t, tradingSymbolDtos, "BTCUSDT").DisplayName)
}

func TestAListingSaysWhichMarketsKeepHoursAndThereforeShut(t *testing.T) {
	// "Trading now" and "keeps hours" coincide during a session, but only the latter tells a closed market from a quiet one.
	fixture := newTradingSymbolServiceUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return([]entities.TradingSymbol{
		{Symbol: "2330", Market: string(vo.MarketTaiwanStock)},
		{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto)},
	}, nil)
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)
	fixture.tradingSymbolRepository.EXPECT().
		FindWatched(gomock.Any()).Return([]entities.TradingSymbol{}, nil)

	tradingSymbolDtos, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

	assert.NoError(t, err)
	assert.True(t, listed(t, tradingSymbolDtos, "2330").HasTradingSession)
	assert.False(t, listed(t, tradingSymbolDtos, "BTCUSDT").HasTradingSession)
}

func TestALimitedMarketOnlyPromisesLiveUpdatesToSymbolsHoldingAPlace(t *testing.T) {
	// Five places, six watched symbols: promising the sixth live updates would give it a chart that never moves.
	fixture := newTradingSymbolServiceUnderTest(t)
	watchlist := make([]entities.TradingSymbol, 0, 6)
	for index, symbol := range []string{"2330", "2454", "2603", "2317", "2412", "1301"} {
		watchlist = append(watchlist, entities.TradingSymbol{
			Symbol:       symbol,
			Market:       "taiwanStock",
			IsWatched:    true,
			RegisteredAt: registrationTime.Add(time.Duration(index) * time.Hour),
		})
	}
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return(watchlist, nil)
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)
	fixture.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(watchlist, nil)

	tradingSymbolDtos, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

	assert.NoError(t, err)
	for _, holdingAPlace := range []string{"2330", "2454", "2603", "2317", "2412"} {
		assert.True(t, listed(t, tradingSymbolDtos, holdingAPlace).HasLiveUpdates, holdingAPlace)
	}
	assert.False(t, listed(t, tradingSymbolDtos, "1301").HasLiveUpdates)
}

func TestAMarketWithNoCeilingAlwaysPromisesLiveUpdates(t *testing.T) {
	// Looking is all it takes to start a follow, so there is nothing to warn about.
	fixture := newTradingSymbolServiceUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return([]entities.TradingSymbol{
		{Symbol: "BTCUSDT", Market: "crypto"},
	}, nil)
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)
	fixture.tradingSymbolRepository.EXPECT().
		FindWatched(gomock.Any()).Return([]entities.TradingSymbol{}, nil)

	tradingSymbolDtos, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

	assert.NoError(t, err)
	assert.True(t, listed(t, tradingSymbolDtos, "BTCUSDT").HasLiveUpdates)
}

func TestASymbolTakenOffTheWatchlistIsStillListed(t *testing.T) {
	// Its candles remain queryable, so it must stay pickable.
	fixture := newTradingSymbolServiceUnderTest(t)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return([]entities.TradingSymbol{
		{Symbol: "2330", Market: "taiwanStock", IsWatched: false, RegisteredAt: registrationTime},
	}, nil)
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{"2330"}, nil)
	fixture.tradingSymbolRepository.EXPECT().
		FindWatched(gomock.Any()).Return([]entities.TradingSymbol{}, nil)

	tradingSymbolDtos, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, []string{"2330"}, namesOfListed(tradingSymbolDtos))
	assert.False(t, listed(t, tradingSymbolDtos, "2330").IsWatched)
	assert.False(t, listed(t, tradingSymbolDtos, "2330").HasLiveUpdates)
}

func TestListingReportsAFailureReadingTheWatchlist(t *testing.T) {
	fixture := newTradingSymbolServiceUnderTest(t)
	storageFailure := errors.New("storage unreachable")
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).
		Return([]entities.TradingSymbol{}, nil)
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)
	fixture.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(nil, storageFailure)

	_, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

	assert.ErrorIs(t, err, storageFailure)
}

// listed picks one symbol from a listing, failing rather than panicking when it is absent.
func listed(
	t *testing.T, tradingSymbolDtos []dto.TradingSymbolDto, symbol string,
) dto.TradingSymbolDto {
	t.Helper()

	for _, tradingSymbolDto := range tradingSymbolDtos {
		if tradingSymbolDto.Symbol == symbol {
			return tradingSymbolDto
		}
	}

	t.Fatalf("%s was not listed at all", symbol)

	return dto.TradingSymbolDto{}
}

func TestAClosedMarketSaysSoOnEveryOneOfItsSymbols(t *testing.T) {
	// Only the service knows the market's days off; the console would call a holiday an outage.
	fixture := newTradingSymbolServiceUnderTestAt(t, closedMarketTime)
	fixture.tradingSymbolRepository.EXPECT().FindAll(gomock.Any()).Return([]entities.TradingSymbol{
		{Symbol: "2330", Market: "taiwanStock", IsWatched: true, RegisteredAt: registrationTime},
		{Symbol: "BTCUSDT", Market: "crypto", IsWatched: true, RegisteredAt: registrationTime},
	}, nil)
	fixture.kCandleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)
	fixture.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).
		Return([]entities.TradingSymbol{}, nil)

	tradingSymbolDtos, err := fixture.tradingSymbolService.ListTradingSymbols(t.Context())

	assert.NoError(t, err)
	assert.False(t, listed(t, tradingSymbolDtos, "2330").IsWithinTradingSession)
	// A market shut for the evening also has no live places to give.
	assert.False(t, listed(t, tradingSymbolDtos, "2330").HasLiveUpdates)
	// The round-the-clock market is untouched by any of it.
	assert.True(t, listed(t, tradingSymbolDtos, "BTCUSDT").IsWithinTradingSession)
	assert.True(t, listed(t, tradingSymbolDtos, "BTCUSDT").HasLiveUpdates)
}
