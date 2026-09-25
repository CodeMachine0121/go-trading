package controller_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const contractRouterSpotStrategyScriptID = uint(31)

type contractIndicatorRouterUnderTest struct {
	engine                       *gin.Engine
	kCandleContractRepository    *mocks.MockIKCandleContractRepository
	contractIndicatorScriptProxy *mocks.MockIContractIndicatorScriptProxy
}

func newContractIndicatorRouterUnderTest(t *testing.T) contractIndicatorRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(indicatorRouterNow).AnyTimes()

	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(mockController)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), contractRouterSpotStrategyScriptID).
		Return(entities.StrategyScript{
			ID: contractRouterSpotStrategyScriptID, OwnerID: signedInViewerID, Script: "the spot script",
			MarketDataKind: "kCandle",
		}, nil).AnyTimes()
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.StrategyScript{}, domains.ErrStrategyScriptNotFound).AnyTimes()
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(mockController)

	kCandleContractRepository := mocks.NewMockIKCandleContractRepository(mockController)
	contractFundingRateSettlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(mockController)
	contractFundingRateSettlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	contractFundingRateSettlementRepository.EXPECT().FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entities.ContractFundingRateSettlement{}, false, nil).AnyTimes()
	contractPositionStatisticRepository := mocks.NewMockIContractPositionStatisticRepository(mockController)
	contractPositionStatisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	contractIndicatorScriptProxy := mocks.NewMockIContractIndicatorScriptProxy(mockController)

	indicatorCalculationController := controller.NewIndicatorCalculationController(
		application.NewIndicatorCalculationApplication(
			service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
			// Nothing here calculates over spot candles.
			nil,
			service.NewContractIndicatorCalculationService(
				kCandleContractRepository, contractFundingRateSettlementRepository, contractPositionStatisticRepository,
				contractIndicatorScriptProxy, clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				queryMaxResults)))

	engine := gin.New()
	engine.POST("/contract-indicator-calculations",
		doorOpenFor(t, signedInViewerID), indicatorCalculationController.CalculateContractIndicator)

	return contractIndicatorRouterUnderTest{
		engine:                       engine,
		kCandleContractRepository:    kCandleContractRepository,
		contractIndicatorScriptProxy: contractIndicatorScriptProxy,
	}
}

func (fixture contractIndicatorRouterUnderTest) post(body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/contract-indicator-calculations", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

func (fixture contractIndicatorRouterUnderTest) expectOneStoredContractCandle() {
	figure := decimal.RequireFromString("100")
	fixture.kCandleContractRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return([]entities.KCandleContract{{
			Symbol: "BTCUSDT", OpenTime: indicatorRouterNow.Add(-2 * time.Minute),
			Open: figure, High: figure, Low: figure, Close: figure,
			MarkOpen: figure, MarkHigh: figure, MarkLow: figure, MarkClose: figure,
		}}, nil)
}

func aContractAlgorithmBody() string {
	return `{"symbol":"BTCUSDT","startTime":"` + indicatorRouterNow.Add(-5*time.Minute).Format(time.RFC3339) +
		`","script":"my own contract script","resultType":"float"}`
}

func TestContractIndicatorRouteAnswersWithTheCalculation(t *testing.T) {
	fixture := newContractIndicatorRouterUnderTest(t)
	fixture.expectOneStoredContractCandle()
	fixture.contractIndicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), "my own contract script", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{"funding": {Numbers: []float64{0.0001}}}, nil)

	recorder := fixture.post(aContractAlgorithmBody())

	require.Equal(t, http.StatusOK, recorder.Code)
	responseBody := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &responseBody))
	assert.Equal(t, 0.0001, responseBody["values"].(map[string]any)["funding"])
	assert.Equal(t, float64(1), responseBody["usedCandleCount"])
}

func TestContractIndicatorRouteRefusesASpotScriptAsTheCallersChoiceToChange(t *testing.T) {
	fixture := newContractIndicatorRouterUnderTest(t)

	recorder := fixture.post(`{"symbol":"BTCUSDT","startTime":"2026-09-02T00:00:00Z","strategyScriptId":31}`)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "這支策略腳本吃的是 K 線")
}

func TestContractIndicatorRouteAnswersLikeTheSpotRouteWhenSomethingGoesWrong(t *testing.T) {
	t.Run("a strategy script that is not there", func(t *testing.T) {
		fixture := newContractIndicatorRouterUnderTest(t)

		recorder := fixture.post(`{"symbol":"BTCUSDT","startTime":"2026-09-02T00:00:00Z","strategyScriptId":99}`)

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("a body naming a strategy script and carrying an algorithm", func(t *testing.T) {
		fixture := newContractIndicatorRouterUnderTest(t)

		recorder := fixture.post(
			`{"symbol":"BTCUSDT","startTime":"2026-09-02T00:00:00Z","strategyScriptId":31,"script":"mine"}`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("a body that cannot be read", func(t *testing.T) {
		fixture := newContractIndicatorRouterUnderTest(t)

		recorder := fixture.post(`{`)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("a stretch too thin to yield a value, with both counts", func(t *testing.T) {
		fixture := newContractIndicatorRouterUnderTest(t)
		fixture.kCandleContractRepository.EXPECT().
			FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

		recorder := fixture.post(aContractAlgorithmBody())

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		responseBody := map[string]any{}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &responseBody))
		assert.Equal(t, float64(0), responseBody["availableCandleCount"])
		assert.Equal(t, float64(1), responseBody["minimumCandleCount"])
	})

	t.Run("a script that fails", func(t *testing.T) {
		fixture := newContractIndicatorRouterUnderTest(t)
		fixture.expectOneStoredContractCandle()
		fixture.contractIndicatorScriptProxy.EXPECT().
			Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, domains.ErrIndicatorScriptFailed)

		recorder := fixture.post(aContractAlgorithmBody())

		assert.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	})
}

func TestContractIndicatorRouteTurnsAwayARequestCarryingNoProofOfIdentity(t *testing.T) {
	// Nothing is stubbed on storage or the script runner: nothing may reach either.
	fixture := newContractIndicatorRouterUnderTest(t)
	request := httptest.NewRequest(http.MethodPost, "/contract-indicator-calculations",
		strings.NewReader(aContractAlgorithmBody()))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	fixture.engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
