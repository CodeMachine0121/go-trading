package controller_test

import (
	"context"
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
	kCandleContractRepository       *mocks.MockIKCandleContractRepository
	contractTradingSymbolRepository *mocks.MockIContractTradingSymbolRepository
	contractIndicatorScriptProxy    *mocks.MockIContractIndicatorScriptProxy
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
		clockProxy, queryMaxResults, time.Minute)
	contractBacktestService := service.NewContractBacktestService(
		kCandleContractRepository, settlementRepository, statisticRepository,
		contractTradingSymbolRepository, tierRepository, contractIndicatorScriptProxy, clockProxy, queryMaxResults, time.Minute)

	backtestController := controller.NewBacktestController(
		application.NewBacktestApplication(strategyScriptService, backtestService, contractBacktestService))
	tradingStrategyBacktestController := controller.NewTradingStrategyBacktestController(
		application.NewTradingStrategyBacktestApplication(
			service.NewTradingStrategyService(tradingStrategyRepository),
			strategyScriptService, backtestService, contractBacktestService))

	engine := gin.New()
	engine.POST("/contract-backtests", doorOpenFor(t, signedInViewerID), backtestController.RunContractBacktest)
	engine.POST("/trading-strategies/:id/contract-backtests", doorOpenFor(t, signedInViewerID),
		tradingStrategyBacktestController.RunContractTradingStrategyBacktest)

	return contractBacktestRouterUnderTest{
		engine:                          engine,
		kCandleContractRepository:       kCandleContractRepository,
		contractTradingSymbolRepository: contractTradingSymbolRepository,
		contractIndicatorScriptProxy:    contractIndicatorScriptProxy,
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

func (fixture contractBacktestRouterUnderTest) expectASpecifiedSymbolWithTwoBars() {
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

	kCandleContracts := make([]entities.KCandleContract, 0, 2)
	for hour, closePrice := range []string{"100", "110"} {
		price := decimal.RequireFromString(closePrice)
		kCandleContracts = append(kCandleContracts, entities.KCandleContract{
			Symbol: "BTCUSDT", OpenTime: backtestRouterStart.Add(time.Duration(hour) * time.Hour),
			Open: price, High: price, Low: price, Close: price,
			MarkOpen: price, MarkHigh: price, MarkLow: price, MarkClose: price,
		})
	}
	fixture.kCandleContractRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(kCandleContracts, nil).AnyTimes()
}

func TestRunContractBacktestEndpoint(t *testing.T) {
	t.Run("a completed contract replay comes back with its contract report card", func(t *testing.T) {
		fixture := newContractBacktestRouterUnderTest(t)
		fixture.expectASpecifiedSymbolWithTwoBars()
		fixture.contractIndicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]map[string]vo.IndicatorValueVo{
				{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}},
				{vo.SignalIndicatorKey: {Signal: vo.SignalSell}},
			}, nil)

		response := fixture.post("/contract-backtests",
			fmt.Sprintf(contractBacktestBody, 9, `,"tradingMode":"longOnly"`))

		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var body struct {
			TradingMode string `json:"tradingMode"`
			Leverage    string `json:"leverage"`
			Summary     struct {
				LiquidationExitCount   int    `json:"liquidationExitCount"`
				TotalFundingFee        string `json:"totalFundingFee"`
				LongTradeCount         int    `json:"longTradeCount"`
				MaintenanceMarginBasis struct {
					Kind string `json:"kind"`
				} `json:"maintenanceMarginBasis"`
			} `json:"summary"`
			ClosedTrades []struct {
				Direction string `json:"direction"`
				Profit    string `json:"profit"`
			} `json:"closedTrades"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		assert.Equal(t, "longOnly", body.TradingMode)
		assert.Equal(t, "5", body.Leverage)
		assert.Equal(t, 1, body.Summary.LongTradeCount)
		assert.Equal(t, "smallestTier", body.Summary.MaintenanceMarginBasis.Kind)
		require.Len(t, body.ClosedTrades, 1)
		assert.Equal(t, "long", body.ClosedTrades[0].Direction)
		assert.Equal(t, "5000", body.ClosedTrades[0].Profit)
	})

	t.Run("a K candle strategy script is refused as the caller's choice, not a script failure", func(t *testing.T) {
		fixture := newContractBacktestRouterUnderTest(t)

		response := fixture.post("/contract-backtests",
			fmt.Sprintf(contractBacktestBody, 10, ""))

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "吃的是 K 線")
	})

	t.Run("a refused figure names the field it came from", func(t *testing.T) {
		fixture := newContractBacktestRouterUnderTest(t)
		fixture.expectASpecifiedSymbolWithTwoBars()

		response := fixture.post("/contract-backtests",
			fmt.Sprintf(contractBacktestBody, 9, `,"slippagePercentage":"-1"`))

		require.Equal(t, http.StatusBadRequest, response.Code)
		var body struct {
			Field string `json:"field"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		assert.Equal(t, "slippage", body.Field)
	})
}

func TestRunContractTradingStrategyBacktestEndpoint(t *testing.T) {
	t.Run("a trading mode sent with the replay is refused naming the field", func(t *testing.T) {
		fixture := newContractBacktestRouterUnderTest(t)
		fixture.expectASpecifiedSymbolWithTwoBars()
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractRouterStrategyID).
			Return(entities.TradingStrategy{
				ID: contractRouterStrategyID, OwnerID: signedInViewerID, Name: "合約",
				MarketDataKind: "contractKCandle", TradingMode: "longShort",
				SignalSources: []entities.TradingStrategySignalSource{{
					Label: "A", StrategyScriptID: contractRouterScriptID, AggregationInterval: "1h",
				}},
				ConditionNodes: []entities.TradingStrategyConditionNode{
					{ID: 1, Side: "buy", SourceLabel: "A", ExpectedSignal: "buy"},
					{ID: 2, Side: "sell", SourceLabel: "A", ExpectedSignal: "sell"},
				},
			}, nil)

		response := fixture.post("/trading-strategies/11/contract-backtests", `{
			"symbol":"BTCUSDT",
			"startTime":"2026-08-29T00:00:00Z",
			"endTime":"2026-08-29T04:00:00Z",
			"initialCapital":"10000",
			"tradingMode":"shortOnly"
		}`)

		require.Equal(t, http.StatusBadRequest, response.Code)
		var body struct {
			Field string `json:"field"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		assert.Equal(t, "tradingMode", body.Field)
	})
}

func TestContractBacktestEndpointsRefuseMalformedRequests(t *testing.T) {
	testCases := []struct {
		name string
		path string
		body string
	}{
		{name: "a contract replay body that is not JSON", path: "/contract-backtests", body: "not json"},
		{
			name: "a contract replay naming a script and carrying one",
			path: "/contract-backtests",
			body: `{"strategyScriptId":9,"script":"written here","symbol":"BTCUSDT"}`,
		},
		{name: "a trading strategy replay body that is not JSON", path: "/trading-strategies/11/contract-backtests", body: "not json"},
		{name: "a trading strategy identifier that is not a number", path: "/trading-strategies/abc/contract-backtests", body: "{}"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newContractBacktestRouterUnderTest(t)

			response := fixture.post(testCase.path, testCase.body)

			assert.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}

func TestRunContractBacktestEndpointCarriesAWrittenScriptAndItsKnobs(t *testing.T) {
	fixture := newContractBacktestRouterUnderTest(t)
	fixture.expectASpecifiedSymbolWithTwoBars()
	fixture.contractIndicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), "written here", gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, _ domains.IndicatorResultTypeDomain,
			_ []vo.ContractKCandleVo, parameters domains.StrategyScriptParametersDomain,
		) ([]map[string]vo.IndicatorValueVo, error) {
			return []map[string]vo.IndicatorValueVo{
				{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
				{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
			}, nil
		})

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

	assert.Equal(t, http.StatusOK, response.Code, response.Body.String())
}

func TestRunContractTradingStrategyBacktestEndpointAnswersWithTheReportCard(t *testing.T) {
	fixture := newContractBacktestRouterUnderTest(t)
	fixture.expectASpecifiedSymbolWithTwoBars()
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractRouterStrategyID).
		Return(entities.TradingStrategy{
			ID: contractRouterStrategyID, OwnerID: signedInViewerID, Name: "合約",
			MarketDataKind: "contractKCandle", TradingMode: "longOnly",
			SignalSources: []entities.TradingStrategySignalSource{{
				Label: "A", StrategyScriptID: contractRouterScriptID, AggregationInterval: "1h",
			}},
			ConditionNodes: []entities.TradingStrategyConditionNode{
				{ID: 1, Side: "buy", SourceLabel: "A", ExpectedSignal: "buy"},
				{ID: 2, Side: "sell", SourceLabel: "A", ExpectedSignal: "sell"},
			},
		}, nil)
	fixture.contractIndicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]map[string]vo.IndicatorValueVo{
			{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}},
			{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
		}, nil)

	response := fixture.post("/trading-strategies/11/contract-backtests", `{
		"symbol":"BTCUSDT",
		"startTime":"2026-08-29T00:00:00Z",
		"endTime":"2026-08-29T04:00:00Z",
		"initialCapital":"10000"
	}`)

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var body struct {
		TradingMode string `json:"tradingMode"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, "longOnly", body.TradingMode)
}

func TestContractBacktestEndpointsNeedASignedInCaller(t *testing.T) {
	for _, path := range []string{"/contract-backtests", "/trading-strategies/11/contract-backtests"} {
		t.Run(path, func(t *testing.T) {
			fixture := newContractBacktestRouterUnderTest(t)
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			fixture.engine.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		})
	}
}

func TestContractBacktestEndpointCarriesFillTimingAndValidationStart(t *testing.T) {
	fixture := newContractBacktestRouterUnderTest(t)
	fixture.expectASpecifiedSymbolWithTwoBars()
	fixture.contractIndicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]map[string]vo.IndicatorValueVo{
			{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}},
			{vo.SignalIndicatorKey: {Signal: vo.SignalHold}},
		}, nil)

	response := fixture.post("/contract-backtests", fmt.Sprintf(contractBacktestBody, 9,
		`,"fillTiming":"nextOpen","validationStartTime":"2026-08-29T01:00:00Z"`))

	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var body struct {
		FillTiming string `json:"fillTiming"`
		InSample   struct {
			UsedCandleCount int `json:"usedCandleCount"`
		} `json:"inSample"`
		Validation struct {
			UsedCandleCount int `json:"usedCandleCount"`
		} `json:"validation"`
		Summary struct {
			MaximumConsecutiveLossCount int      `json:"maximumConsecutiveLossCount"`
			ProfitFactor                *float64 `json:"profitFactor"`
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, "nextOpen", body.FillTiming)
	assert.Equal(t, 1, body.InSample.UsedCandleCount)
	assert.Equal(t, 1, body.Validation.UsedCandleCount)
	assert.Nil(t, body.Summary.ProfitFactor)
}

func TestContractBacktestEndpointRefusesAnUnknownFillTiming(t *testing.T) {
	fixture := newContractBacktestRouterUnderTest(t)
	fixture.expectASpecifiedSymbolWithTwoBars()

	response := fixture.post("/contract-backtests", fmt.Sprintf(contractBacktestBody, 9, `,"fillTiming":"intraday"`))

	require.Equal(t, http.StatusBadRequest, response.Code)
	var body struct {
		Field string `json:"field"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, "fillTiming", body.Field)
}

func TestContractBacktestEndpointsAnswerARunOutAllowanceAsUnprocessable(t *testing.T) {
	for _, path := range []string{"/contract-backtests", "/trading-strategies/11/contract-backtests"} {
		t.Run(path, func(t *testing.T) {
			fixture := newContractBacktestRouterUnderTest(t)
			fixture.expectASpecifiedSymbolWithTwoBars()
			fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), contractRouterStrategyID).
				Return(entities.TradingStrategy{
					ID: contractRouterStrategyID, OwnerID: signedInViewerID, Name: "合約",
					MarketDataKind: "contractKCandle",
					SignalSources: []entities.TradingStrategySignalSource{{
						Label: "A", StrategyScriptID: contractRouterScriptID, AggregationInterval: "1h",
					}},
					ConditionNodes: []entities.TradingStrategyConditionNode{
						{ID: 1, Side: "buy", SourceLabel: "A", ExpectedSignal: "buy"},
						{ID: 2, Side: "sell", SourceLabel: "A", ExpectedSignal: "sell"},
					},
				}, nil).AnyTimes()
			fixture.contractIndicatorScriptProxy.EXPECT().
				ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, domains.BacktestTimeAllowanceSpent(90*time.Second))

			response := fixture.post(path, fmt.Sprintf(contractBacktestBody, 9, ""))

			assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
			assert.Contains(t, response.Body.String(), "沒跑完")
		})
	}
}
