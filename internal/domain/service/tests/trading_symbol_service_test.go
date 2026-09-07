package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
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
}

func newTradingSymbolServiceUnderTest(t *testing.T) tradingSymbolServiceUnderTest {
	controller := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(registrationTime).AnyTimes()

	return tradingSymbolServiceUnderTest{
		tradingSymbolService: service.NewTradingSymbolService(
			tradingSymbolRepository, kCandleRepository, clockProxy),
		tradingSymbolRepository: tradingSymbolRepository,
		kCandleRepository:       kCandleRepository,
	}
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
