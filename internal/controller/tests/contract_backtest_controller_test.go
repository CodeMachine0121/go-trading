package controller_test

import (
	"encoding/json"
	"fmt"
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

const (
	contractRouterScriptID        = uint(9)
	contractRouterKCandleScriptID = uint(10)
	contractRouterStrategyID      = uint(11)
)

const contractBacktestBody = `{
	"symbol":"BTCUSDT",
	"aggregationInterval":"1h",
	"startTime":"2026-08-29T00:00:00Z",
	"endTime":"2026-08-29T04:00:00Z",
	"strategyScriptId":%d,
	"initialCapital":"10000",
	"leverage":"5"
	%s
}`

type contractBacktestRouterUnderTest struct {
	engine                          *gin.Engine
	runs                            routerBacktestRuns
	contractTradingSymbolRepository *mocks.MockIContractTradingSymbolRepository
	tradingStrategyRepository       *mocks.MockITradingStrategyRepository
}

func newContractBacktestRouterUnderTest(t *testing.T) contractBacktestRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(backtestRouterNow).AnyTimes()

	kCandleContractRepository := mocks.NewMockIKCandleContractRepository(mockController)
	settlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(mockController)
	settlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.ContractFundingRateSettlement{}, nil).AnyTimes()
	settlementRepository.EXPECT().FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entities.ContractFundingRateSettlement{}, false, nil).AnyTimes()
	statisticRepository := mocks.NewMockIContractPositionStatisticRepository(mockController)
	statisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.ContractPositionStatistic{}, nil).AnyTimes()
	contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	tierRepository := mocks.NewMockIContractMaintenanceMarginTierRepository(mockController)
	tierRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return([]entities.ContractMaintenanceMarginTier{}, nil).AnyTimes()
	contractIndicatorScriptProxy := mocks.NewMockIContractIndicatorScriptProxy(mockController)

	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(mockController)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), contractRouterScriptID).
		Return(entities.StrategyScript{
			ID: contractRouterScriptID, OwnerID: signedInViewerID, Script: "the script",
			MarketDataKind: string(vo.MarketDataKindContractKCandle),
		}, nil).AnyTimes()
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), contractRouterKCandleScriptID).
		Return(entities.StrategyScript{
			ID: contractRouterKCandleScriptID, OwnerID: signedInViewerID, Script: "the script",
			MarketDataKind: string(vo.MarketDataKindKCandle),
		}, nil).AnyTimes()
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()
	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(mockController)

	strategyScriptService := service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository)
	backtestService := service.NewBacktestService(
		mocks.NewMockIKCandleRepository(mockController), mocks.NewMockIIndicatorScriptProxy(mockController),
		clockProxy, queryMaxResults, 0)
	contractBacktestService := service.NewContractBacktestService(
		kCandleContractRepository, settlementRepository, statisticRepository,
		contractTradingSymbolRepository, tierRepository, contractIndicatorScriptProxy, noOrderFlowService(t), clockProxy, queryMaxResults, 0)

	runs := newRouterBacktestRuns(t, mockController, clockProxy, backtestService, contractBacktestService)

	backtestController := controller.NewBacktestController(application.NewBacktestApplication(
		strategyScriptService, backtestService, contractBacktestService, runs.backtestRunService))
	tradingStrategyBacktestController := controller.NewTradingStrategyBacktestController(
		application.NewTradingStrategyBacktestApplication(
			service.NewTradingStrategyService(tradingStrategyRepository),
			strategyScriptService, backtestService, contractBacktestService, runs.backtestRunService, time.Minute))

	engine := gin.New()
	engine.POST("/contract-backtests", doorOpenFor(t, signedInViewerID), backtestController.SubmitContractBacktest)
	engine.POST("/trading-strategies/:id/contract-backtests", doorOpenFor(t, signedInViewerID),
		tradingStrategyBacktestController.SubmitContractTradingStrategyBacktest)

	return contractBacktestRouterUnderTest{
		engine:                          engine,
		runs:                            runs,
		contractTradingSymbolRepository: contractTradingSymbolRepository,
		tradingStrategyRepository:       tradingStrategyRepository,
	}
}

func (fixture contractBacktestRouterUnderTest) post(path string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

func (fixture contractBacktestRouterUnderTest) expectASpecifiedSymbol() {
	confirmedAt := backtestRouterStart
	fundingIntervalHours := 8
	fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{
			Symbol:                 "BTCUSDT",
			QuantityStep:           decimal.NewNullDecimal(decimal.RequireFromString("0.001")),
			MaintenanceMarginRate:  decimal.NewNullDecimal(decimal.RequireFromString("0.005")),
			FundingIntervalHours:   &fundingIntervalHours,
			SpecificationUpdatedAt: &confirmedAt,
		}, true, nil).AnyTimes()
}

// aContractRouterStrategy is the signed-in viewer's one-source contract strategy under the given trading mode.
func aContractRouterStrategy(tradingMode string) entities.TradingStrategy {
	return entities.TradingStrategy{
		ID: contractRouterStrategyID, OwnerID: signedInViewerID, Name: "合約",
		MarketDataKind: "contractKCandle", TradingMode: tradingMode,
		SignalSources: []entities.TradingStrategySignalSource{{
			Label: "A", StrategyScriptID: contractRouterScriptID, AggregationInterval: "1h",
		}},
		ConditionNodes: []entities.TradingStrategyConditionNode{
			{ID: 1, Side: "buy", SourceLabel: "A", ExpectedSignal: "buy"},
			{ID: 2, Side: "sell", SourceLabel: "A", ExpectedSignal: "sell"},
		},
	}
}

const contractTradingStrategyBacktestBody = `{
	"symbol":"BTCUSDT",
	"startTime":"2026-08-29T00:00:00Z",
	"endTime":"2026-08-29T04:00:00Z",
	"initialCapital":"10000",
	"leverage":"5"
	%s
}`

func TestSubmitContractBacktestEndpoint(t *testing.T) {
	t.Run("a valid contract replay is queued with its leverage and trading mode", func(t *testing.T) {
		fixture := newContractBacktestRouterUnderTest(t)
		fixture.expectASpecifiedSymbol()
		saved := fixture.runs.expectQueued()

		response := fixture.post("/contract-backtests", fmt.Sprintf(contractBacktestBody, 9,
			`,"tradingMode":"longOnly","fillTiming":"nextOpen","validationStartTime":"2026-08-29T01:00:00Z"`))

		require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
		body := map[string]any{}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		assert.Equal(t, "contractScript", body["kind"])
		assert.Equal(t, "queued", body["status"])
		assert.Equal(t, "5", body["leverage"])
		assert.Equal(t, "longOnly", body["tradingMode"])
		assert.Equal(t, float64(contractRouterScriptID), body["strategyScriptId"])
		assert.NotContains(t, body, "result")
		require.NotNil(t, saved.Request.ContractScript)
		assert.Equal(t, "nextOpen", saved.Request.ContractScript.FillTiming)
		assert.True(t, backtestRouterStart.Add(time.Hour).Equal(saved.Request.ContractScript.ValidationStartTime))
	})

	t.Run("a written script is queued as written, knobs included", func(t *testing.T) {
		fixture := newContractBacktestRouterUnderTest(t)
		fixture.expectASpecifiedSymbol()
		saved := fixture.runs.expectQueued()

		response := fixture.post("/contract-backtests", `{
			"symbol":"BTCUSDT",
			"aggregationInterval":"1h",
			"startTime":"2026-08-29T00:00:00Z",
			"endTime":"2026-08-29T04:00:00Z",
			"script":"written here",
			"parameters":[{"name":"回看根數","kind":"lookbackCount","defaultValue":20}],
			"parameterValues":[{"name":"回看根數","value":5}],
			"initialCapital":"10000"
		}`)

		require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
		assert.Equal(t, "written here", saved.Request.ContractScript.Script)
		assert.Equal(t, 5.0, saved.Request.ContractScript.ParameterValues[0].Value)
		assert.Nil(t, saved.StrategyScriptID)
	})

	t.Run("a contract trading strategy is queued under its own trading mode and name", func(t *testing.T) {
		fixture := newContractBacktestRouterUnderTest(t)
		fixture.expectASpecifiedSymbol()
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractRouterStrategyID).
			Return(aContractRouterStrategy("longOnly"), nil)
		fixture.runs.expectQueued()

		response := fixture.post("/trading-strategies/11/contract-backtests",
			fmt.Sprintf(contractTradingStrategyBacktestBody, ""))

		require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
		body := map[string]any{}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		assert.Equal(t, "contractTradingStrategy", body["kind"])
		assert.Equal(t, "longOnly", body["tradingMode"])
		assert.Equal(t, float64(contractRouterStrategyID), body["tradingStrategyId"])
		assert.Nil(t, body["strategyScriptId"])
		assert.Equal(t, "合約", body["subjectName"])
	})
}

func TestSubmitContractBacktestEndpointRefusesBeforeQueueing(t *testing.T) {
	testCases := []struct {
		name            string
		path            string
		body            string
		expectedStatus  int
		expectedField   string
		expectedMessage string
	}{
		{
			name: "a K candle strategy script is the caller's choice, not a script failure",
			path: "/contract-backtests", body: fmt.Sprintf(contractBacktestBody, 10, ""),
			expectedStatus: http.StatusBadRequest, expectedMessage: "吃的是 K 線",
		},
		{
			name: "a refused figure names the field it came from",
			path: "/contract-backtests", body: fmt.Sprintf(contractBacktestBody, 9, `,"slippagePercentage":"-1"`),
			expectedStatus: http.StatusBadRequest, expectedField: "slippage",
		},
		{
			name: "an unknown fill timing names its field",
			path: "/contract-backtests", body: fmt.Sprintf(contractBacktestBody, 9, `,"fillTiming":"intraday"`),
			expectedStatus: http.StatusBadRequest, expectedField: "fillTiming",
		},
		{
			name:           "a trading mode sent with a trading strategy replay names the field",
			path:           "/trading-strategies/11/contract-backtests",
			body:           fmt.Sprintf(contractTradingStrategyBacktestBody, `,"tradingMode":"shortOnly"`),
			expectedStatus: http.StatusBadRequest, expectedField: "tradingMode",
		},
		{name: "a contract replay body that is not JSON", path: "/contract-backtests", body: "not json",
			expectedStatus: http.StatusBadRequest},
		{
			name: "a contract replay naming a script and carrying one", path: "/contract-backtests",
			body:           `{"strategyScriptId":9,"script":"written here","symbol":"BTCUSDT"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{name: "a trading strategy replay body that is not JSON", path: "/trading-strategies/11/contract-backtests",
			body: "not json", expectedStatus: http.StatusBadRequest},
		{name: "a trading strategy identifier that is not a number", path: "/trading-strategies/abc/contract-backtests",
			body: "{}", expectedStatus: http.StatusBadRequest},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newContractBacktestRouterUnderTest(t)
			fixture.expectASpecifiedSymbol()
			fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractRouterStrategyID).
				Return(aContractRouterStrategy("longShort"), nil).AnyTimes()
			fixture.runs.ownerHasActive(0)

			response := fixture.post(testCase.path, testCase.body)

			require.Equal(t, testCase.expectedStatus, response.Code, response.Body.String())
			body := struct {
				Message string `json:"message"`
				Field   string `json:"field"`
			}{}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			assert.Equal(t, testCase.expectedField, body.Field)
			assert.Contains(t, body.Message, testCase.expectedMessage)
		})
	}
}

func TestSubmitContractTradingStrategyBacktestEndpointForSomebodyElsesStrategy(t *testing.T) {
	fixture := newContractBacktestRouterUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractRouterStrategyID).
		Return(entities.TradingStrategy{}, domains.TradingStrategyNotFound(contractRouterStrategyID))

	response := fixture.post("/trading-strategies/11/contract-backtests", fmt.Sprintf(contractTradingStrategyBacktestBody, ""))

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestSubmitContractBacktestEndpointsAtTheOwnerLimit(t *testing.T) {
	for _, path := range []string{"/contract-backtests", "/trading-strategies/11/contract-backtests"} {
		t.Run(path, func(t *testing.T) {
			fixture := newContractBacktestRouterUnderTest(t)
			fixture.expectASpecifiedSymbol()
			fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractRouterStrategyID).
				Return(aContractRouterStrategy("longShort"), nil).AnyTimes()
			fixture.runs.ownerHasActive(routerBacktestRunOwnerActiveLimit)

			body := fmt.Sprintf(contractBacktestBody, 9, "")
			if path != "/contract-backtests" {
				body = fmt.Sprintf(contractTradingStrategyBacktestBody, "")
			}
			response := fixture.post(path, body)

			require.Equal(t, http.StatusTooManyRequests, response.Code)
			assert.JSONEq(t, `{"message":"你同時最多 3 筆回測在排隊或進行中，等其中一筆結束再送","ownerActiveLimit":3}`,
				response.Body.String())
		})
	}
}

func TestContractBacktestEndpointsNeedASignedInCaller(t *testing.T) {
	for _, path := range []string{"/contract-backtests", "/trading-strategies/11/contract-backtests"} {
		t.Run(path, func(t *testing.T) {
			fixture := newContractBacktestRouterUnderTest(t)

			response := requestWithoutProof(fixture.engine, http.MethodPost, path, "{}")

			assert.Equal(t, http.StatusUnauthorized, response.Code)
		})
	}
}
