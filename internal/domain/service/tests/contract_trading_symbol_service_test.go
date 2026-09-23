package service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type contractSymbolUnderTest struct {
	service          *service.ContractTradingSymbolService
	symbolRepository *mocks.MockIContractTradingSymbolRepository
	candleRepository *mocks.MockIKCandleContractRepository
	lookupProxy      *mocks.MockIContractSymbolLookupProxy
}

func newContractSymbolUnderTest(t *testing.T) contractSymbolUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	candleRepository := mocks.NewMockIKCandleContractRepository(mockController)
	lookupProxy := mocks.NewMockIContractSymbolLookupProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(specificationConfirmedAt).AnyTimes()

	return contractSymbolUnderTest{
		service: service.NewContractTradingSymbolService(
			symbolRepository, candleRepository, lookupProxy, clockProxy),
		symbolRepository: symbolRepository,
		candleRepository: candleRepository,
		lookupProxy:      lookupProxy,
	}
}

func TestContractTradingSymbolServiceListsRegisteredAndHeldTogetherOnce(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).Return(
		[]entities.ContractTradingSymbol{
			{Symbol: "BTCUSDT", IsWatched: true},
			{Symbol: "ETHUSDT", IsWatched: false},
		}, nil)
	underTest.candleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return(
		[]string{"BTCUSDT", "1000PEPEUSDT"}, nil)

	contractSymbols, listError := underTest.service.ListContractTradingSymbols(t.Context())

	require.NoError(t, listError)
	require.Len(t, contractSymbols, 3)
	assert.Equal(t, "1000PEPEUSDT", contractSymbols[0].Symbol)
	assert.False(t, contractSymbols[0].IsWatched)
	// Known only because candles are held for it: never registered, so never given
	// a trading specification.
	assert.Nil(t, contractSymbols[0].TradingSpecification)
	assert.Equal(t, "BTCUSDT", contractSymbols[1].Symbol)
	assert.True(t, contractSymbols[1].IsWatched)
	assert.Equal(t, "ETHUSDT", contractSymbols[2].Symbol)
	assert.False(t, contractSymbols[2].IsWatched)
}

func TestContractTradingSymbolServiceAddsAListedContract(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "1000PEPEUSDT").
		Return(vo.ContractSymbolListingVo{IsListed: true}, nil)
	underTest.symbolRepository.EXPECT().Save(gomock.Any(), entities.ContractTradingSymbol{
		Symbol: "1000PEPEUSDT", IsWatched: true,
	}).Return(nil)

	addError := underTest.service.AddToWatchlist(t.Context(), "1000PEPEUSDT")

	assert.NoError(t, addError)
}

func TestContractTradingSymbolServiceRefusesAContractTheVenueDoesNotList(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "NOSUCHPAIR").
		Return(vo.ContractSymbolListingVo{}, nil)
	underTest.symbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	addError := underTest.service.AddToWatchlist(t.Context(), "NOSUCHPAIR")

	assert.ErrorIs(t, addError, domains.ErrTradingSymbolNotInMarket)
}

func TestContractTradingSymbolServiceAddsNothingWhenTheVenueCannotBeReached(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "BTCUSDT").
		Return(vo.ContractSymbolListingVo{}, sourceUnreachable)
	underTest.symbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	addError := underTest.service.AddToWatchlist(t.Context(), "BTCUSDT")

	assert.ErrorIs(t, addError, domains.ErrMarketDataSourceUnavailable)
}

func TestContractTradingSymbolServiceRefusesANameItCannotRead(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)

	addError := underTest.service.AddToWatchlist(t.Context(), "   ")
	removeError := underTest.service.RemoveFromWatchlist(t.Context(), "   ")

	assert.ErrorIs(t, addError, domains.ErrWatchlistEntryValidation)
	assert.ErrorIs(t, removeError, domains.ErrWatchlistEntryValidation)
}

func TestContractTradingSymbolServiceStopsFollowingWithoutForgettingTheContract(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil)
	underTest.symbolRepository.EXPECT().Save(gomock.Any(), entities.ContractTradingSymbol{
		Symbol: "BTCUSDT", IsWatched: false,
	}).Return(nil)

	removeError := underTest.service.RemoveFromWatchlist(t.Context(), "BTCUSDT")

	assert.NoError(t, removeError)
}

func TestContractTradingSymbolServiceTreatsRemovingWhatIsNotFollowedAsDone(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.ContractTradingSymbol{}, false, nil)
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "ETHUSDT").Return(
		entities.ContractTradingSymbol{Symbol: "ETHUSDT", IsWatched: false}, true, nil)
	underTest.symbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	unregisteredError := underTest.service.RemoveFromWatchlist(t.Context(), "BTCUSDT")
	alreadyStoppedError := underTest.service.RemoveFromWatchlist(t.Context(), "ETHUSDT")

	assert.NoError(t, unregisteredError)
	assert.NoError(t, alreadyStoppedError)
}

func TestContractTradingSymbolServicePassesStorageFailuresOn(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).Return(nil, sourceUnreachable)
	underTest.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{}, false, sourceUnreachable)

	_, listError := underTest.service.ListContractTradingSymbols(t.Context())
	removeError := underTest.service.RemoveFromWatchlist(t.Context(), "BTCUSDT")

	assert.ErrorIs(t, listError, sourceUnreachable)
	assert.ErrorIs(t, removeError, sourceUnreachable)
}

func TestContractTradingSymbolServicePassesACandleStorageFailureOn(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).Return(
		[]entities.ContractTradingSymbol{}, nil)
	underTest.candleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).
		Return(nil, sourceUnreachable)

	_, listError := underTest.service.ListContractTradingSymbols(t.Context())

	assert.ErrorIs(t, listError, sourceUnreachable)
}

var specificationConfirmedAt = time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)

var assertAnError = errors.New("storage unreachable")

func intervalOf(hours int) *int {
	return &hours
}

func reportedSpecificationOf(symbol string, tickSize string, fundingIntervalHours *int) vo.ContractTradingSpecificationVo {
	return vo.ContractTradingSpecificationVo{
		Symbol:                symbol,
		TickSize:              decimal.RequireFromString(tickSize),
		QuantityStep:          decimal.RequireFromString("0.001"),
		MinimumQuantity:       decimal.RequireFromString("0.001"),
		MinimumNotional:       decimal.RequireFromString("50"),
		MaintenanceMarginRate: decimal.RequireFromString("0.025"),
		LiquidationFeeRate:    decimal.RequireFromString("0.0125"),
		FundingIntervalHours:  fundingIntervalHours,
	}
}

func TestContractTradingSymbolServiceRecordsTheSpecificationWhenAddingAContract(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "BTCUSDT").Return(vo.ContractSymbolListingVo{
		IsListed: true, Specification: reportedSpecificationOf("BTCUSDT", "0.1", nil),
	}, nil)
	underTest.symbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, saved entities.ContractTradingSymbol) error {
			assert.Equal(t, "BTCUSDT", saved.Symbol)
			assert.True(t, saved.IsWatched)
			assert.True(t, decimal.RequireFromString("0.1").Equal(saved.TickSize.Decimal))
			assert.True(t, decimal.RequireFromString("0.001").Equal(saved.QuantityStep.Decimal))
			assert.True(t, decimal.RequireFromString("50").Equal(saved.MinimumNotional.Decimal))
			require.NotNil(t, saved.FundingIntervalHours)
			assert.Equal(t, 8, *saved.FundingIntervalHours)
			require.NotNil(t, saved.SpecificationUpdatedAt)
			assert.Equal(t, specificationConfirmedAt, *saved.SpecificationUpdatedAt)

			return nil
		})

	addError := underTest.service.AddToWatchlist(t.Context(), "BTCUSDT")

	assert.NoError(t, addError)
}

func TestContractTradingSymbolServiceRefreshesEveryKnownContractTheVenueStillLists(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).Return([]entities.ContractTradingSymbol{
		{Symbol: "BTCUSDT", IsWatched: true, TickSize: decimal.NewNullDecimal(decimal.RequireFromString("0.1"))},
		{Symbol: "ETHUSDT", IsWatched: false},
		{Symbol: "OMGUSDT", IsWatched: false},
	}, nil)
	underTest.lookupProxy.EXPECT().FetchTradingSpecifications(gomock.Any()).Return(
		[]vo.ContractTradingSpecificationVo{
			reportedSpecificationOf("BTCUSDT", "0.01", nil),
			reportedSpecificationOf("ETHUSDT", "0.01", intervalOf(4)),
			reportedSpecificationOf("NOTREGISTEREDUSDT", "0.01", nil),
		}, nil)
	underTest.symbolRepository.EXPECT().SaveTradingSpecifications(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, refreshed []entities.ContractTradingSymbol) error {
			require.Len(t, refreshed, 2)
			assert.Equal(t, "BTCUSDT", refreshed[0].Symbol)
			assert.True(t, refreshed[0].IsWatched)
			assert.True(t, decimal.RequireFromString("0.01").Equal(refreshed[0].TickSize.Decimal))
			assert.Equal(t, 8, *refreshed[0].FundingIntervalHours)
			assert.Equal(t, specificationConfirmedAt, *refreshed[0].SpecificationUpdatedAt)
			assert.Equal(t, "ETHUSDT", refreshed[1].Symbol)
			assert.False(t, refreshed[1].IsWatched)
			assert.Equal(t, 4, *refreshed[1].FundingIntervalHours)

			return nil
		})

	refreshedCount, refreshError := underTest.service.RefreshTradingSpecifications(t.Context())

	require.NoError(t, refreshError)
	assert.Equal(t, 2, refreshedCount)
}

func TestContractTradingSymbolServiceRefreshLeavesOutASpecificationThatCannotBeOne(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).Return(
		[]entities.ContractTradingSymbol{{Symbol: "BTCUSDT"}, {Symbol: "ETHUSDT"}}, nil)
	underTest.lookupProxy.EXPECT().FetchTradingSpecifications(gomock.Any()).Return(
		[]vo.ContractTradingSpecificationVo{
			reportedSpecificationOf("BTCUSDT", "0", nil),
			reportedSpecificationOf("ETHUSDT", "0.01", nil),
		}, nil)
	underTest.symbolRepository.EXPECT().SaveTradingSpecifications(gomock.Any(), gomock.Len(1)).Return(nil)

	refreshedCount, refreshError := underTest.service.RefreshTradingSpecifications(t.Context())

	require.NoError(t, refreshError)
	assert.Equal(t, 1, refreshedCount)
}

func TestContractTradingSymbolServiceRefreshWritesNothingWhenNothingIsListed(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).Return(
		[]entities.ContractTradingSymbol{{Symbol: "OMGUSDT"}}, nil)
	underTest.lookupProxy.EXPECT().FetchTradingSpecifications(gomock.Any()).Return(
		[]vo.ContractTradingSpecificationVo{reportedSpecificationOf("BTCUSDT", "0.1", nil)}, nil)

	refreshedCount, refreshError := underTest.service.RefreshTradingSpecifications(t.Context())

	require.NoError(t, refreshError)
	assert.Equal(t, 0, refreshedCount)
}

func TestContractTradingSymbolServiceRefreshChangesNothingWhenItCannotFinish(t *testing.T) {
	testCases := []struct {
		name          string
		arrange       func(underTest contractSymbolUnderTest)
		expectedError error
	}{
		{name: "來源不答話", arrange: func(underTest contractSymbolUnderTest) {
			underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).
				Return([]entities.ContractTradingSymbol{{Symbol: "BTCUSDT"}}, nil)
			underTest.lookupProxy.EXPECT().FetchTradingSpecifications(gomock.Any()).
				Return(nil, errors.New("venue unreachable"))
		}, expectedError: domains.ErrMarketDataSourceUnavailable},
		{name: "讀不到認得哪些合約", arrange: func(underTest contractSymbolUnderTest) {
			underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).Return(nil, assertAnError)
		}, expectedError: assertAnError},
		{name: "存不進去", arrange: func(underTest contractSymbolUnderTest) {
			underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).
				Return([]entities.ContractTradingSymbol{{Symbol: "BTCUSDT"}}, nil)
			underTest.lookupProxy.EXPECT().FetchTradingSpecifications(gomock.Any()).
				Return([]vo.ContractTradingSpecificationVo{reportedSpecificationOf("BTCUSDT", "0.1", nil)}, nil)
			underTest.symbolRepository.EXPECT().SaveTradingSpecifications(gomock.Any(), gomock.Any()).
				Return(assertAnError)
		}, expectedError: assertAnError},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newContractSymbolUnderTest(t)
			testCase.arrange(underTest)

			refreshedCount, refreshError := underTest.service.RefreshTradingSpecifications(t.Context())

			assert.ErrorIs(t, refreshError, testCase.expectedError)
			assert.Equal(t, 0, refreshedCount)
		})
	}
}

func TestContractTradingSymbolServiceListsEachContractWithItsSpecification(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	fourHours := 4
	underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).Return([]entities.ContractTradingSymbol{
		{Symbol: "BTCUSDT", IsWatched: true,
			TickSize:               decimal.NewNullDecimal(decimal.RequireFromString("0.1")),
			QuantityStep:           decimal.NewNullDecimal(decimal.RequireFromString("0.001")),
			MinimumQuantity:        decimal.NewNullDecimal(decimal.RequireFromString("0.001")),
			MinimumNotional:        decimal.NewNullDecimal(decimal.RequireFromString("50")),
			MaintenanceMarginRate:  decimal.NewNullDecimal(decimal.RequireFromString("0.025")),
			LiquidationFeeRate:     decimal.NewNullDecimal(decimal.RequireFromString("0.0125")),
			FundingIntervalHours:   &fourHours,
			SpecificationUpdatedAt: &specificationConfirmedAt},
		{Symbol: "ETHUSDT", IsWatched: true},
	}, nil)
	underTest.candleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)

	listed, listError := underTest.service.ListContractTradingSymbols(t.Context())

	require.NoError(t, listError)
	require.Len(t, listed, 2)
	require.NotNil(t, listed[0].TradingSpecification)
	assert.True(t, decimal.RequireFromString("0.1").Equal(listed[0].TradingSpecification.TickSize))
	assert.True(t, decimal.RequireFromString("0.001").Equal(listed[0].TradingSpecification.QuantityStep))
	assert.True(t, decimal.RequireFromString("0.001").Equal(listed[0].TradingSpecification.MinimumQuantity))
	assert.True(t, decimal.RequireFromString("50").Equal(listed[0].TradingSpecification.MinimumNotional))
	assert.True(t, decimal.RequireFromString("0.025").Equal(listed[0].TradingSpecification.MaintenanceMarginRate))
	assert.True(t, decimal.RequireFromString("0.0125").Equal(listed[0].TradingSpecification.LiquidationFeeRate))
	assert.Equal(t, 4, listed[0].TradingSpecification.FundingIntervalHours)
	assert.Equal(t, specificationConfirmedAt, listed[0].TradingSpecification.SpecificationUpdatedAt)
	assert.Nil(t, listed[1].TradingSpecification)
}
