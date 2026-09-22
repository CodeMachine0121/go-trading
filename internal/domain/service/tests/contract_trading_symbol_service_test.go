package service_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
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

	return contractSymbolUnderTest{
		service: service.NewContractTradingSymbolService(
			symbolRepository, candleRepository, lookupProxy),
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
	assert.Equal(t, "BTCUSDT", contractSymbols[1].Symbol)
	assert.True(t, contractSymbols[1].IsWatched)
	assert.Equal(t, "ETHUSDT", contractSymbols[2].Symbol)
	assert.False(t, contractSymbols[2].IsWatched)
}

func TestContractTradingSymbolServiceAddsAListedContract(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "1000PEPEUSDT").
		Return(vo.SymbolListingVo{IsListed: true}, nil)
	underTest.symbolRepository.EXPECT().Save(gomock.Any(), entities.ContractTradingSymbol{
		Symbol: "1000PEPEUSDT", IsWatched: true,
	}).Return(nil)

	addError := underTest.service.AddToWatchlist(t.Context(), "1000PEPEUSDT")

	assert.NoError(t, addError)
}

func TestContractTradingSymbolServiceRefusesAContractTheVenueDoesNotList(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "NOSUCHPAIR").
		Return(vo.SymbolListingVo{}, nil)
	underTest.symbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	addError := underTest.service.AddToWatchlist(t.Context(), "NOSUCHPAIR")

	assert.ErrorIs(t, addError, domains.ErrTradingSymbolNotInMarket)
}

func TestContractTradingSymbolServiceAddsNothingWhenTheVenueCannotBeReached(t *testing.T) {
	underTest := newContractSymbolUnderTest(t)
	underTest.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "BTCUSDT").
		Return(vo.SymbolListingVo{}, sourceUnreachable)
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
