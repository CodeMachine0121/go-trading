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

var ladderConfirmedAt = time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)

func ladderTier(tier int, floor string, cap string, rate string) vo.ContractMaintenanceMarginTierVo {
	return vo.ContractMaintenanceMarginTierVo{
		Tier: tier, NotionalFloor: decimal.RequireFromString(floor), NotionalCap: decimal.RequireFromString(cap),
		MaintenanceMarginRate: decimal.RequireFromString(rate), MaintenanceAmount: decimal.Zero, MaximumLeverage: 50,
	}
}

type marginTierServiceUnderTest struct {
	service          *service.ContractMaintenanceMarginTierService
	tierRepository   *mocks.MockIContractMaintenanceMarginTierRepository
	symbolRepository *mocks.MockIContractTradingSymbolRepository
	tierProxy        *mocks.MockIContractMaintenanceMarginTierProxy
}

func newMarginTierServiceUnderTest(t *testing.T) marginTierServiceUnderTest {
	t.Helper()

	mockController := gomock.NewController(t)
	tierRepository := mocks.NewMockIContractMaintenanceMarginTierRepository(mockController)
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	tierProxy := mocks.NewMockIContractMaintenanceMarginTierProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(ladderConfirmedAt).AnyTimes()

	return marginTierServiceUnderTest{
		service:          service.NewContractMaintenanceMarginTierService(tierRepository, symbolRepository, tierProxy, clockProxy),
		tierRepository:   tierRepository,
		symbolRepository: symbolRepository,
		tierProxy:        tierProxy,
	}
}

func (underTest marginTierServiceUnderTest) registered(symbols ...string) {
	registeredSymbols := make([]entities.ContractTradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		registeredSymbols = append(registeredSymbols, entities.ContractTradingSymbol{Symbol: symbol})
	}
	underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).Return(registeredSymbols, nil)
}

func TestMaintenanceMarginRefreshReplacesTheLadderOfEveryRegisteredContractReported(t *testing.T) {
	underTest := newMarginTierServiceUnderTest(t)
	underTest.tierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).Return(
		[]vo.ContractMaintenanceMarginLadderVo{
			{Symbol: "BTCUSDT", Tiers: []vo.ContractMaintenanceMarginTierVo{
				ladderTier(1, "0", "50000", "0.004"), ladderTier(2, "50000", "250000", "0.005")}},
			{Symbol: "ETHUSDT", Tiers: []vo.ContractMaintenanceMarginTierVo{ladderTier(1, "0", "10000", "0.0065")}},
			{Symbol: "1000PEPEUSDT", Tiers: []vo.ContractMaintenanceMarginTierVo{ladderTier(1, "0", "5000", "0.01")}},
		}, nil)
	// OMGUSDT is registered but not reported: it is not named, so it keeps its ladder.
	underTest.registered("BTCUSDT", "ETHUSDT", "OMGUSDT")
	underTest.tierRepository.EXPECT().ReplaceLadders(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, laddersBySymbol map[string][]entities.ContractMaintenanceMarginTier) error {
			require.Len(t, laddersBySymbol, 2)
			require.Len(t, laddersBySymbol["BTCUSDT"], 2)
			assert.Equal(t, 2, laddersBySymbol["BTCUSDT"][1].Tier)
			assert.Equal(t, ladderConfirmedAt, laddersBySymbol["BTCUSDT"][1].ConfirmedAt)
			assert.True(t, decimal.RequireFromString("0.0065").Equal(laddersBySymbol["ETHUSDT"][0].MaintenanceMarginRate))
			assert.NotContains(t, laddersBySymbol, "OMGUSDT")
			assert.NotContains(t, laddersBySymbol, "1000PEPEUSDT")

			return nil
		})

	refreshReport, refreshError := underTest.service.RefreshLadders(t.Context())

	require.NoError(t, refreshError)
	assert.Equal(t, 2, refreshReport.RefreshedCount)
	assert.Empty(t, refreshReport.RefusedLadders)
}

func TestMaintenanceMarginRefreshKeepsOnlyTheContractWhoseLadderCannotBeOne(t *testing.T) {
	underTest := newMarginTierServiceUnderTest(t)
	underTest.tierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).Return(
		[]vo.ContractMaintenanceMarginLadderVo{
			{Symbol: "BTCUSDT", Tiers: []vo.ContractMaintenanceMarginTierVo{
				ladderTier(1, "0", "50000", "0.004"), ladderTier(2, "40000", "250000", "0.005")}},
			{Symbol: "ETHUSDT", Tiers: []vo.ContractMaintenanceMarginTierVo{ladderTier(1, "0", "10000", "0.0065")}},
		}, nil)
	underTest.registered("BTCUSDT", "ETHUSDT")
	underTest.tierRepository.EXPECT().ReplaceLadders(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, laddersBySymbol map[string][]entities.ContractMaintenanceMarginTier) error {
			assert.NotContains(t, laddersBySymbol, "BTCUSDT")
			assert.Contains(t, laddersBySymbol, "ETHUSDT")

			return nil
		})

	refreshReport, refreshError := underTest.service.RefreshLadders(t.Context())

	require.NoError(t, refreshError)
	assert.Equal(t, 1, refreshReport.RefreshedCount)
	require.Len(t, refreshReport.RefusedLadders, 1)
	assert.Equal(t, "BTCUSDT", refreshReport.RefusedLadders[0].Symbol)
	assert.Contains(t, refreshReport.RefusedLadders[0].Reason, "分級重疊")
}

func TestMaintenanceMarginRefreshWritesNothingWhenNoRegisteredContractIsReported(t *testing.T) {
	underTest := newMarginTierServiceUnderTest(t)
	underTest.tierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).Return(
		[]vo.ContractMaintenanceMarginLadderVo{{Symbol: "ETHUSDT", Tiers: []vo.ContractMaintenanceMarginTierVo{
			ladderTier(1, "0", "10000", "0.0065")}}}, nil)
	underTest.registered("BTCUSDT")

	refreshReport, refreshError := underTest.service.RefreshLadders(t.Context())

	require.NoError(t, refreshError)
	assert.Equal(t, 0, refreshReport.RefreshedCount)
}

func TestMaintenanceMarginRefreshChangesNothingWhenItCannotFinish(t *testing.T) {
	testCases := []struct {
		name          string
		arrange       func(underTest marginTierServiceUnderTest)
		expectedError error
	}{
		{name: "沒有帳戶金鑰", arrange: func(underTest marginTierServiceUnderTest) {
			underTest.tierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).
				Return(nil, domains.ErrContractAccountCredentialsMissing)
		}, expectedError: domains.ErrContractAccountCredentialsMissing},
		{name: "金鑰被拒", arrange: func(underTest marginTierServiceUnderTest) {
			underTest.tierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).
				Return(nil, domains.ErrContractAccountCredentialsRefused)
		}, expectedError: domains.ErrContractAccountCredentialsRefused},
		{name: "來源不答話", arrange: func(underTest marginTierServiceUnderTest) {
			underTest.tierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).
				Return(nil, errors.New("venue unreachable"))
		}, expectedError: domains.ErrMarketDataSourceUnavailable},
		{name: "讀不到認得哪些合約", arrange: func(underTest marginTierServiceUnderTest) {
			underTest.tierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).Return(nil, nil)
			underTest.symbolRepository.EXPECT().FindAll(gomock.Any()).Return(nil, assertAnError)
		}, expectedError: assertAnError},
		{name: "存不進去", arrange: func(underTest marginTierServiceUnderTest) {
			underTest.tierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).Return(
				[]vo.ContractMaintenanceMarginLadderVo{{Symbol: "BTCUSDT", Tiers: []vo.ContractMaintenanceMarginTierVo{
					ladderTier(1, "0", "50000", "0.004")}}}, nil)
			underTest.registered("BTCUSDT")
			underTest.tierRepository.EXPECT().ReplaceLadders(gomock.Any(), gomock.Any()).Return(assertAnError)
		}, expectedError: assertAnError},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newMarginTierServiceUnderTest(t)
			testCase.arrange(underTest)

			refreshReport, refreshError := underTest.service.RefreshLadders(t.Context())

			assert.ErrorIs(t, refreshError, testCase.expectedError)
			assert.Equal(t, 0, refreshReport.RefreshedCount)
		})
	}
}

func TestMaintenanceMarginTiersReadBack(t *testing.T) {
	underTest := newMarginTierServiceUnderTest(t)
	underTest.tierRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		[]entities.ContractMaintenanceMarginTier{
			{Symbol: "BTCUSDT", Tier: 1, NotionalCap: decimal.RequireFromString("50000"), ConfirmedAt: ladderConfirmedAt},
			{Symbol: "BTCUSDT", Tier: 2, NotionalCap: decimal.RequireFromString("250000"), ConfirmedAt: ladderConfirmedAt},
		}, nil)

	tiers, findError := underTest.service.FindTiers(t.Context(), " btcusdt ")

	require.NoError(t, findError)
	require.Len(t, tiers, 2)
	assert.Equal(t, 1, tiers[0].Tier)
	assert.Equal(t, 2, tiers[1].Tier)
	assert.Equal(t, ladderConfirmedAt, tiers[1].ConfirmedAt)
}

func TestMaintenanceMarginTiersRefuseAQueryWithoutAContract(t *testing.T) {
	underTest := newMarginTierServiceUnderTest(t)

	_, findError := underTest.service.FindTiers(t.Context(), " ")

	assert.ErrorIs(t, findError, domains.ErrContractMaintenanceMarginTierValidation)
	assert.ErrorContains(t, findError, "必須指定交易標的")
}

func TestMaintenanceMarginTiersPassAStorageFailureOn(t *testing.T) {
	underTest := newMarginTierServiceUnderTest(t)
	underTest.tierRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(nil, assertAnError)

	_, findError := underTest.service.FindTiers(t.Context(), "BTCUSDT")

	assert.ErrorIs(t, findError, assertAnError)
}
