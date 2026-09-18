package controller_test

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type strategyBotRouterUnderTest struct {
	engine                     *gin.Engine
	strategyBotRepository      *mocks.MockIStrategyBotRepository
	strategyScriptRepository   *mocks.MockIStrategyScriptRepository
	tradingStrategyRepository  *mocks.MockITradingStrategyRepository
	telegramDeliveryRepository *mocks.MockITelegramDeliveryRepository
}

func newStrategyBotRouterUnderTest(t *testing.T) strategyBotRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)

	strategyBotRepository := mocks.NewMockIStrategyBotRepository(mockController)
	strategyBotRunRecordRepository := mocks.NewMockIStrategyBotRunRecordRepository(mockController)
	strategyBotRunRecordRepository.EXPECT().
		Append(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	strategyBotRunRecordRepository.EXPECT().
		FindLatestByBot(gomock.Any(), gomock.Any()).
		Return([]entities.StrategyBotRunRecord{}, nil).AnyTimes()
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(mockController)
	telegramDeliveryRepository := mocks.NewMockITelegramDeliveryRepository(mockController)

	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)).AnyTimes()

	secretSealProxy := mocks.NewMockISecretSealProxy(mockController)
	secretSealProxy.EXPECT().Unseal(gomock.Any()).Return("the-token", nil).AnyTimes()
	messageDeliveryProxy := mocks.NewMockIMessageDeliveryProxy(mockController)
	messageDeliveryProxy.EXPECT().Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(vo.DeliveryFailureNone, nil).AnyTimes()

	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	marketCatalog := domains.NewMarketCatalogDomain(
		map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}})

	// 兩邊共用同一組 service：一台機器人只有一份狀態，兩份會讓這幾個測試
	// 在「按了按鈕之後那台變成什麼樣」上對不起來。
	strategyBotService := service.NewStrategyBotService(
		strategyBotRepository, strategyBotRunRecordRepository, clockProxy)
	strategyScriptService := service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository)
	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(mockController)
	tradingStrategyService := service.NewTradingStrategyService(tradingStrategyRepository)
	// 啟動與停止會讓機器人說一句它自己的動靜；這幾個測試問的是路由與狀態碼，
	// 所以整條投遞路徑一律放行。
	telegramDeliveryService := service.NewTelegramDeliveryService(
		telegramDeliveryRepository, secretSealProxy, messageDeliveryProxy)

	strategyBotController := controller.NewStrategyBotController(
		application.NewStrategyBotApplication(
			strategyBotService, tradingStrategyService, telegramDeliveryService),
		// 「立即運算」那一條走時鐘那一側，而它走的必須是同一條路。
		application.NewStrategyBotRunApplication(
			strategyBotService,
			tradingStrategyService,
			strategyScriptService,
			service.NewIndicatorCalculationService(
				kCandleRepository, tradingSymbolRepository,
				mocks.NewMockIIndicatorScriptProxy(mockController),
				clockProxy, marketCatalog, 1000),
			telegramDeliveryService,
			service.NewKCandleService(
				kCandleRepository, tradingSymbolRepository, clockProxy, marketCatalog, 1000),
			clockProxy,
			application.NewStrategyBotRoundGuard(),
			4,
			time.Minute,
		))

	engine := gin.New()
	requiresSignIn := doorOpenFor(t, signedInViewerID)
	engine.POST("/strategy-bots", requiresSignIn, strategyBotController.CreateStrategyBot)
	engine.GET("/strategy-bots", requiresSignIn, strategyBotController.ListStrategyBots)
	engine.GET("/strategy-bots/:id", requiresSignIn, strategyBotController.GetStrategyBot)
	engine.PUT("/strategy-bots/:id", requiresSignIn, strategyBotController.UpdateStrategyBot)
	engine.DELETE("/strategy-bots/:id", requiresSignIn, strategyBotController.DeleteStrategyBot)
	engine.POST("/strategy-bots/:id/power", requiresSignIn, strategyBotController.StartStrategyBot)
	engine.DELETE("/strategy-bots/:id/power", requiresSignIn, strategyBotController.StopStrategyBot)
	engine.POST("/strategy-bots/:id/runs", requiresSignIn, strategyBotController.RunRoundNow)

	return strategyBotRouterUnderTest{
		engine:                     engine,
		strategyBotRepository:      strategyBotRepository,
		strategyScriptRepository:   strategyScriptRepository,
		tradingStrategyRepository:  tradingStrategyRepository,
		telegramDeliveryRepository: telegramDeliveryRepository,
	}
}

func (fixture strategyBotRouterUnderTest) send(
	method string, target string, body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

// aStrategyBotBody is one bot: a name, a market, how often, and the rules it names.
// The rules are not in the body — a bot names a set, it does not carry one.
const aStrategyBotBody = `{
	"name": "早盤突破",
	"symbol": "BTCUSDT",
	"tradingStrategyId": 9,
	"triggerIntervalMinutes": 5
}`

// expectResolvableTradingStrategy is the one question saving a bot asks of anything
// outside itself: may this person use the rules they named?
func (fixture strategyBotRouterUnderTest) expectResolvableTradingStrategy() {
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(entities.TradingStrategy{
			ID: 9, OwnerID: signedInViewerID, Name: "黃金交叉",
		}, nil).AnyTimes()
}

func aStoredStrategyBotRow(runState vo.StrategyBotRunStateVo) entities.StrategyBot {
	return entities.StrategyBot{
		ID: 3, OwnerID: signedInViewerID, Name: "早盤突破", Symbol: "BTCUSDT",
		TradingStrategyID:      9,
		TradingStrategy:        entities.TradingStrategy{ID: 9, Name: "黃金交叉"},
		TriggerIntervalMinutes: 5,
		RunState:               string(runState),
	}
}

func TestStrategyBotRouterCreatesABotAndAnswersWithIt(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)
	fixture.expectResolvableTradingStrategy()

	fixture.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
			// The rules a person named arrived as a name and nothing else.
			assert.Equal(t, uint(9), bot.TradingStrategyID)
			// The owner comes from the proof of identity, never from the body.
			assert.Equal(t, signedInViewerID, bot.OwnerID)

			return aStoredStrategyBotRow(vo.StrategyBotStopped), nil
		})

	response := fixture.send(http.MethodPost, "/strategy-bots", aStrategyBotBody)

	require.Equal(t, http.StatusCreated, response.Code)
	answer := map[string]any{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Equal(t, "早盤突破", answer["name"])
	assert.Equal(t, string(vo.StrategyBotStopped), answer["runState"])
	// The rules' current name comes back beside their identifier, so a list says
	// what each bot is doing without a second call per bot.
	assert.Equal(t, "黃金交叉", answer["tradingStrategyName"])
	// A bot's owner is nothing a person reading their own bots learns from.
	assert.NotContains(t, answer, "ownerId")
}

func TestStrategyBotRouterListsAndReadsBots(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)

	fixture.strategyBotRepository.EXPECT().FindAllByOwner(gomock.Any(), signedInViewerID).
		Return([]entities.StrategyBot{aStoredStrategyBotRow(vo.StrategyBotRunning)}, nil)
	fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
		Return(aStoredStrategyBotRow(vo.StrategyBotRunning), nil)

	listResponse := fixture.send(http.MethodGet, "/strategy-bots", "")
	require.Equal(t, http.StatusOK, listResponse.Code)
	listed := []map[string]any{}
	require.NoError(t, json.Unmarshal(listResponse.Body.Bytes(), &listed))
	require.Len(t, listed, 1)
	assert.Equal(t, string(vo.StrategyBotRunning), listed[0]["runState"])

	getResponse := fixture.send(http.MethodGet, "/strategy-bots/3", "")
	assert.Equal(t, http.StatusOK, getResponse.Code)
}

func TestStrategyBotRouterRewritesAndDeletes(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)
	fixture.expectResolvableTradingStrategy()

	fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
		Return(aStoredStrategyBotRow(vo.StrategyBotStopped), nil).Times(2)
	fixture.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(aStoredStrategyBotRow(vo.StrategyBotStopped), nil)
	fixture.strategyBotRepository.EXPECT().Delete(gomock.Any(), uint(3)).Return(nil)

	updateResponse := fixture.send(http.MethodPut, "/strategy-bots/3", aStrategyBotBody)
	assert.Equal(t, http.StatusOK, updateResponse.Code)

	deleteResponse := fixture.send(http.MethodDelete, "/strategy-bots/3", "")
	assert.Equal(t, http.StatusNoContent, deleteResponse.Code)
}

func TestStrategyBotRouterStartsAndStops(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)

	fixture.telegramDeliveryRepository.EXPECT().FindOneByUser(gomock.Any(), signedInViewerID).
		Return(entities.TelegramDelivery{UserID: signedInViewerID}, nil).AnyTimes()
	fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
		Return(aStoredStrategyBotRow(vo.StrategyBotStopped), nil)
	fixture.strategyBotRepository.EXPECT().
		CountRunningByOwner(gomock.Any(), signedInViewerID).Return(0, nil)
	fixture.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	startResponse := fixture.send(http.MethodPost, "/strategy-bots/3/power", "")
	require.Equal(t, http.StatusOK, startResponse.Code)
	assert.Contains(t, startResponse.Body.String(), string(vo.StrategyBotRunning))

	fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
		Return(aStoredStrategyBotRow(vo.StrategyBotRunning), nil)
	fixture.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	stopResponse := fixture.send(http.MethodDelete, "/strategy-bots/3/power", "")
	require.Equal(t, http.StatusOK, stopResponse.Code)
	assert.Contains(t, stopResponse.Body.String(), string(vo.StrategyBotStopped))
}

func TestStrategyBotRouterMapsEachRefusalOntoItsOwnStatus(t *testing.T) {
	testCases := []struct {
		name           string
		arrange        func(fixture strategyBotRouterUnderTest)
		method         string
		target         string
		body           string
		expectedStatus int
	}{
		{
			name:           "an identifier that is not a positive number",
			arrange:        func(strategyBotRouterUnderTest) {},
			method:         http.MethodGet,
			target:         "/strategy-bots/0",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "a body that is not readable",
			arrange:        func(strategyBotRouterUnderTest) {},
			method:         http.MethodPost,
			target:         "/strategy-bots",
			body:           "{",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "a bot that breaks a rule",
			arrange: func(fixture strategyBotRouterUnderTest) {
				fixture.expectResolvableTradingStrategy()
			},
			method:         http.MethodPost,
			target:         "/strategy-bots",
			body:           `{"name": "", "symbol": "BTCUSDT", "triggerIntervalMinutes": 5}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "a bot that is not this person's",
			arrange: func(fixture strategyBotRouterUnderTest) {
				strangersBot := aStoredStrategyBotRow(vo.StrategyBotStopped)
				strangersBot.OwnerID = signedInViewerID + 1
				fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
					Return(strangersBot, nil)
			},
			method:         http.MethodGet,
			target:         "/strategy-bots/3",
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "starting with nowhere to be spoken to",
			arrange: func(fixture strategyBotRouterUnderTest) {
				fixture.telegramDeliveryRepository.EXPECT().
					FindOneByUser(gomock.Any(), signedInViewerID).
					Return(entities.TelegramDelivery{}, domains.ErrTelegramDeliveryNotConfigured)
				fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
					Return(aStoredStrategyBotRow(vo.StrategyBotStopped), nil)
				fixture.strategyBotRepository.EXPECT().
					CountRunningByOwner(gomock.Any(), signedInViewerID).Return(0, nil)
			},
			method:         http.MethodPost,
			target:         "/strategy-bots/3/power",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "rewriting a bot that is running",
			arrange: func(fixture strategyBotRouterUnderTest) {
				fixture.expectResolvableTradingStrategy()
				fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
					Return(aStoredStrategyBotRow(vo.StrategyBotRunning), nil)
			},
			method:         http.MethodPut,
			target:         "/strategy-bots/3",
			body:           aStrategyBotBody,
			expectedStatus: http.StatusConflict,
		},
		{
			name: "a name this person already uses",
			arrange: func(fixture strategyBotRouterUnderTest) {
				fixture.expectResolvableTradingStrategy()
				fixture.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					Return(entities.StrategyBot{}, domains.ErrStrategyBotNameConflict)
			},
			method:         http.MethodPost,
			target:         "/strategy-bots",
			body:           aStrategyBotBody,
			expectedStatus: http.StatusConflict,
		},
		{
			name: "starting at the running limit",
			arrange: func(fixture strategyBotRouterUnderTest) {
				fixture.telegramDeliveryRepository.EXPECT().
					FindOneByUser(gomock.Any(), signedInViewerID).
					Return(entities.TelegramDelivery{UserID: signedInViewerID}, nil)
				fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
					Return(aStoredStrategyBotRow(vo.StrategyBotStopped), nil)
				fixture.strategyBotRepository.EXPECT().
					CountRunningByOwner(gomock.Any(), signedInViewerID).Return(10, nil)
			},
			method:         http.MethodPost,
			target:         "/strategy-bots/3/power",
			expectedStatus: http.StatusConflict,
		},
		{
			name: "storage that could not answer",
			arrange: func(fixture strategyBotRouterUnderTest) {
				fixture.strategyBotRepository.EXPECT().
					FindAllByOwner(gomock.Any(), signedInViewerID).
					Return(nil, errors.New("the database went away"))
			},
			method:         http.MethodGet,
			target:         "/strategy-bots",
			expectedStatus: http.StatusBadGateway,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyBotRouterUnderTest(t)
			testCase.arrange(fixture)

			response := fixture.send(testCase.method, testCase.target, testCase.body)

			assert.Equal(t, testCase.expectedStatus, response.Code)
		})
	}
}

func TestStrategyBotRouterRefusesEveryRouteWithoutProofOfIdentity(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)

	targets := []struct {
		method string
		target string
	}{
		{http.MethodPost, "/strategy-bots"},
		{http.MethodGet, "/strategy-bots"},
		{http.MethodGet, "/strategy-bots/3"},
		{http.MethodPut, "/strategy-bots/3"},
		{http.MethodDelete, "/strategy-bots/3"},
		{http.MethodPost, "/strategy-bots/3/power"},
		{http.MethodDelete, "/strategy-bots/3/power"},
	}

	for _, target := range targets {
		t.Run(target.method+" "+target.target, func(t *testing.T) {
			request := httptest.NewRequest(target.method, target.target, strings.NewReader(""))
			recorder := httptest.NewRecorder()
			fixture.engine.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		})
	}
}

func TestStrategyBotRouterRefusesAnUnreadableIdentifierOnEveryRouteThatTakesOne(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)

	targets := []struct {
		method string
		target string
		body   string
	}{
		{http.MethodGet, "/strategy-bots/abc", ""},
		{http.MethodPut, "/strategy-bots/abc", aStrategyBotBody},
		{http.MethodDelete, "/strategy-bots/abc", ""},
		{http.MethodPost, "/strategy-bots/abc/power", ""},
		{http.MethodDelete, "/strategy-bots/abc/power", ""},
		{http.MethodPut, "/strategy-bots/0", aStrategyBotBody},
		{http.MethodDelete, "/strategy-bots/0", ""},
		{http.MethodPost, "/strategy-bots/0/power", ""},
		{http.MethodDelete, "/strategy-bots/0/power", ""},
	}

	for _, target := range targets {
		t.Run(target.method+" "+target.target, func(t *testing.T) {
			response := fixture.send(target.method, target.target, target.body)

			assert.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}

func TestStrategyBotRouterRefusesAnUnreadableBodyOnARewrite(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)

	response := fixture.send(http.MethodPut, "/strategy-bots/3", "{")

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestStrategyBotRouterReportsStorageThatCouldNotAnswerOnEveryRoute(t *testing.T) {
	testCases := []struct {
		name    string
		arrange func(fixture strategyBotRouterUnderTest)
		method  string
		target  string
		body    string
	}{
		{
			name: "reading one",
			arrange: func(fixture strategyBotRouterUnderTest) {
				fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
					Return(entities.StrategyBot{}, errors.New("the database went away"))
			},
			method: http.MethodGet, target: "/strategy-bots/3",
		},
		{
			name: "deleting one",
			arrange: func(fixture strategyBotRouterUnderTest) {
				fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
					Return(aStoredStrategyBotRow(vo.StrategyBotStopped), nil)
				fixture.strategyBotRepository.EXPECT().Delete(gomock.Any(), uint(3)).
					Return(errors.New("the database went away"))
			},
			method: http.MethodDelete, target: "/strategy-bots/3",
		},
		{
			name: "rewriting one",
			arrange: func(fixture strategyBotRouterUnderTest) {
				fixture.expectResolvableTradingStrategy()
				fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
					Return(aStoredStrategyBotRow(vo.StrategyBotStopped), nil)
				fixture.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					Return(entities.StrategyBot{}, errors.New("the database went away"))
			},
			method: http.MethodPut, target: "/strategy-bots/3", body: aStrategyBotBody,
		},
		{
			name: "starting one",
			arrange: func(fixture strategyBotRouterUnderTest) {
				fixture.telegramDeliveryRepository.EXPECT().
					FindOneByUser(gomock.Any(), signedInViewerID).
					Return(entities.TelegramDelivery{UserID: signedInViewerID}, nil)
				fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
					Return(entities.StrategyBot{}, errors.New("the database went away"))
			},
			method: http.MethodPost, target: "/strategy-bots/3/power",
		},
		{
			name: "stopping one",
			arrange: func(fixture strategyBotRouterUnderTest) {
				fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
					Return(aStoredStrategyBotRow(vo.StrategyBotRunning), nil)
				fixture.strategyBotRepository.EXPECT().
					UpdateRunState(gomock.Any(), gomock.Any()).
					Return(errors.New("the database went away"))
			},
			method: http.MethodDelete, target: "/strategy-bots/3/power",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyBotRouterUnderTest(t)
			testCase.arrange(fixture)

			response := fixture.send(testCase.method, testCase.target, testCase.body)

			assert.Equal(t, http.StatusBadGateway, response.Code)
		})
	}
}

// aPositionPlannedStrategyBotBody is the same bot, saying what it should suggest
// putting down each round.
const aPositionPlannedStrategyBotBody = `{
	"name": "早盤突破",
	"symbol": "BTCUSDT",
	"tradingStrategyId": 9,
	"triggerIntervalMinutes": 5,
	"positionPlan": {
		"capital": "50000",
		"sizingMode": "percentage",
		"sizingValue": "10",
		"leverage": "3",
		"stopLossPercentage": "3",
		"takeProfitPercentage": "5"
	}
}`

func TestStrategyBotRouterCarriesThePositionPlanInAndBackOut(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)
	fixture.expectResolvableTradingStrategy()

	fixture.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
			assert.Equal(t, "50000", bot.PositionPlanCapital.String())
			assert.Equal(t, "percentage", bot.PositionPlanSizingMode)
			assert.Equal(t, "10", bot.PositionPlanSizingValue.String())
			assert.Equal(t, "3", bot.PositionPlanLeverage.String())
			assert.Equal(t, "3", bot.PositionPlanStopLossPercentage.String())
			assert.Equal(t, "5", bot.PositionPlanTakeProfitPercentage.String())

			storedRow := aStoredStrategyBotRow(vo.StrategyBotStopped)
			storedRow.PositionPlanCapital = bot.PositionPlanCapital
			storedRow.PositionPlanSizingMode = bot.PositionPlanSizingMode
			storedRow.PositionPlanSizingValue = bot.PositionPlanSizingValue
			storedRow.PositionPlanLeverage = bot.PositionPlanLeverage
			storedRow.PositionPlanStopLossPercentage = bot.PositionPlanStopLossPercentage
			storedRow.PositionPlanTakeProfitPercentage = bot.PositionPlanTakeProfitPercentage

			return storedRow, nil
		})

	response := fixture.send(
		http.MethodPost, "/strategy-bots", aPositionPlannedStrategyBotBody)

	require.Equal(t, http.StatusCreated, response.Code)

	// It leaves in the answer too: a screen offering to edit this bot has to show the
	// five figures it is currently suggesting from.
	answer := struct {
		PositionPlan struct {
			Capital            string `json:"capital"`
			SizingMode         string `json:"sizingMode"`
			StopLossPercentage string `json:"stopLossPercentage"`
		} `json:"positionPlan"`
	}{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Equal(t, "50000", answer.PositionPlan.Capital)
	assert.Equal(t, "percentage", answer.PositionPlan.SizingMode)
	assert.Equal(t, "3", answer.PositionPlan.StopLossPercentage)
}

func TestStrategyBotRouterRefusesAPositionPlanItCannotUse(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)
	fixture.expectResolvableTradingStrategy()

	// Nothing is stored: gomock enforces it by having no expectation for Save.
	response := fixture.send(http.MethodPost, "/strategy-bots", strings.Replace(
		aPositionPlannedStrategyBotBody, `"sizingValue": "10"`, `"sizingValue": "150"`, 1))

	// The same status every other refused bot gets, because it carries the same
	// sentinel — no controller learned a second one.
	require.Equal(t, http.StatusBadRequest, response.Code)
	// The replay's own sentence, carried through — and no mention of a backtest,
	// which is not what this person was doing.
	assert.Contains(t, response.Body.String(), "百分比必須大於零且不超過一百")
	assert.NotContains(t, response.Body.String(), "backtest")
}

// Leaving the whole group out is an ordinary thing to do: such a bot suggests nothing
// and sends the message it sent before position plans existed.
func TestStrategyBotRouterAcceptsABotWithNoPositionPlan(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)
	fixture.expectResolvableTradingStrategy()

	fixture.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
			assert.True(t, bot.PositionPlanCapital.IsZero())

			return aStoredStrategyBotRow(vo.StrategyBotStopped), nil
		})

	response := fixture.send(http.MethodPost, "/strategy-bots", aStrategyBotBody)

	require.Equal(t, http.StatusCreated, response.Code)
}
