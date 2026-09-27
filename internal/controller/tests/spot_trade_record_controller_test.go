package controller_test

import (
	"context"
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

type spotTradeRouterUnderTest struct {
	engine                         *gin.Engine
	spotTradeRecordRepository      *mocks.MockISpotTradeRecordRepository
	tradeTagRepository             *mocks.MockITradeTagRepository
	tradingStrategyRepository      *mocks.MockITradingStrategyRepository
	strategyBotRunRecordRepository *mocks.MockIStrategyBotRunRecordRepository
	strategyBotRepository          *mocks.MockIStrategyBotRepository
}

func newSpotTradeRouterUnderTest(t *testing.T) spotTradeRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(tradeRouterNow).AnyTimes()
	fixture := spotTradeRouterUnderTest{
		spotTradeRecordRepository:      mocks.NewMockISpotTradeRecordRepository(mockController),
		tradeTagRepository:             mocks.NewMockITradeTagRepository(mockController),
		tradingStrategyRepository:      mocks.NewMockITradingStrategyRepository(mockController),
		strategyBotRunRecordRepository: mocks.NewMockIStrategyBotRunRecordRepository(mockController),
		strategyBotRepository:          mocks.NewMockIStrategyBotRepository(mockController),
	}
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Market: "taiwanStock"}, true, nil).AnyTimes()
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	kCandleRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	kCandleRepository.EXPECT().FindPriceExtremesInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(vo.PriceExtremesVo{}, nil).AnyTimes()
	fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	journalService := service.NewSpotTradeJournalService(
		fixture.spotTradeRecordRepository, fixture.tradeTagRepository, fixture.tradingStrategyRepository,
		tradingSymbolRepository, kCandleRepository, clockProxy)
	recordController := controller.NewSpotTradeRecordController(
		application.NewSpotTradeJournalApplication(journalService, service.NewTradeJournalLinkService(
			mocks.NewMockIOpaqueIdentifierProxy(mockController), fixture.strategyBotRunRecordRepository,
			fixture.strategyBotRepository, "https://console.example")),
		application.NewSpotTradeLiveComparisonApplication(
			journalService, service.NewTradingStrategyService(fixture.tradingStrategyRepository),
			service.NewStrategyScriptService(
				mocks.NewMockIStrategyScriptRepository(mockController),
				mocks.NewMockIPublishedStrategyScriptRepository(mockController)),
			service.NewBacktestService(kCandleRepository, mocks.NewMockIIndicatorScriptProxy(mockController),
				clockProxy, 1000, time.Minute)),
	)

	requiresSignIn := doorOpenFor(t, signedInViewerID)
	engine := gin.New()
	engine.POST("/spot-trade-records", requiresSignIn, recordController.RecordTrade)
	engine.GET("/spot-trade-records", requiresSignIn, recordController.ListTrades)
	engine.GET("/spot-trade-records/statistics", requiresSignIn, recordController.GetStatistics)
	engine.GET("/spot-trade-records/journal-links/:identifier", requiresSignIn, recordController.PrepareJournalLink)
	engine.GET("/spot-trade-records/:id", requiresSignIn, recordController.GetTrade)
	engine.DELETE("/spot-trade-records/:id", requiresSignIn, recordController.DeleteTrade)
	engine.POST("/spot-trade-records/:id/fills", requiresSignIn, recordController.AddFill)
	engine.PUT("/spot-trade-records/:id/fills/:fillId", requiresSignIn, recordController.AmendFill)
	engine.DELETE("/spot-trade-records/:id/fills/:fillId", requiresSignIn, recordController.RemoveFill)
	engine.PUT("/spot-trade-records/:id/plan", requiresSignIn, recordController.AmendPlan)
	engine.POST("/spot-trade-records/:id/notes", requiresSignIn, recordController.AddNote)
	engine.PUT("/spot-trade-records/:id/review", requiresSignIn, recordController.WriteReview)
	engine.PUT("/spot-trade-records/:id/setup-tags", requiresSignIn, recordController.AssignSetupTags)
	engine.GET("/trading-strategies/:id/spot-trade-comparison", requiresSignIn, recordController.CompareWithBacktest)
	fixture.engine = engine

	return fixture
}

func (fixture spotTradeRouterUnderTest) send(method string, path string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	response := httptest.NewRecorder()
	fixture.engine.ServeHTTP(response, request)

	return response
}

func aHeldSpotTradeOf(ownerID uint) entities.SpotTradeRecord {
	boughtAt := tradeRouterNow.Add(-time.Hour)

	return entities.SpotTradeRecord{
		ID: 5, OwnerID: ownerID, Symbol: "2330", Market: "taiwanStock", Status: "open", OpenedAt: boughtAt,
		Fills: []entities.SpotTradeFill{
			{ID: 1, Kind: "buy", FilledAt: boughtAt, Price: decimal.RequireFromString("1050"), Quantity: decimal.RequireFromString("1000")},
			{ID: 2, Kind: "buy", FilledAt: boughtAt, Price: decimal.RequireFromString("1060"), Quantity: decimal.RequireFromString("100")},
		},
	}
}

func echoingSpotSave(_ context.Context, record entities.SpotTradeRecord) (entities.SpotTradeRecord, error) {
	return record, nil
}

func TestSpotTradeRouterRecordTrade(t *testing.T) {
	t.Run("a trade is recorded and answered with the json field names", func(t *testing.T) {
		fixture := newSpotTradeRouterUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindOpenByOwnerSymbol(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.SpotTradeRecord{}, false, nil)
		fixture.spotTradeRecordRepository.EXPECT().Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, record entities.SpotTradeRecord) (entities.SpotTradeRecord, error) {
				record.ID = 31
				return record, nil
			})

		response := fixture.send(http.MethodPost, "/spot-trade-records", `{
			"symbol": "2330",
			"firstBuyFill": {"filledAt": "2026-09-27T09:00:00Z", "price": "1050", "quantity": "1000"},
			"plan": {"plannedStopLossPrice": "1000", "confidence": 3}
		}`)

		require.Equal(t, http.StatusCreated, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `"id":31`)
		assert.Contains(t, body, `"market":"taiwanStock"`)
		assert.Contains(t, body, `"currency":"TWD"`)
		assert.Contains(t, body, `"holding":"1000"`)
		assert.Contains(t, body, `"plannedStopLossPrice":"1000"`)
		assert.Contains(t, body, `"kind":"buy"`)
		assert.NotContains(t, body, "leverage")
		assert.NotContains(t, body, "funding")
	})

	t.Run("a body that is not json, a leverage and a second holding answer 400, 400 and 409", func(t *testing.T) {
		fixture := newSpotTradeRouterUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindOpenByOwnerSymbol(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(aHeldSpotTradeOf(signedInViewerID), true, nil)

		notJson := fixture.send(http.MethodPost, "/spot-trade-records", `{`)
		leverage := fixture.send(http.MethodPost, "/spot-trade-records",
			`{"symbol":"2330","leverage":"10","firstBuyFill":{"price":"1","quantity":"1"}}`)
		second := fixture.send(http.MethodPost, "/spot-trade-records",
			`{"symbol":"2330","firstBuyFill":{"price":"1050","quantity":"1000"}}`)

		assert.Equal(t, http.StatusBadRequest, notJson.Code)
		assert.Equal(t, http.StatusBadRequest, leverage.Code)
		assert.Contains(t, leverage.Body.String(), "現貨只有先買後賣，沒有槓桿")
		assert.Equal(t, http.StatusConflict, second.Code)
		assert.Contains(t, second.Body.String(), `"openTradeId":5`)
	})

	t.Run("a racing second holding answers 409 without an identifier", func(t *testing.T) {
		fixture := newSpotTradeRouterUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindOpenByOwnerSymbol(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.SpotTradeRecord{}, false, nil)
		fixture.spotTradeRecordRepository.EXPECT().Create(gomock.Any(), gomock.Any()).
			Return(entities.SpotTradeRecord{}, domains.SpotTradeOpenHoldingExists("2330", 0))

		response := fixture.send(http.MethodPost, "/spot-trade-records", `{"symbol":"2330","firstBuyFill":{"price":"1","quantity":"1"}}`)

		assert.Equal(t, http.StatusConflict, response.Code)
		assert.NotContains(t, response.Body.String(), "openTradeId")
	})
}

func TestSpotTradeRouterReads(t *testing.T) {
	t.Run("the list narrows by the query and falls back on an unreadable limit", func(t *testing.T) {
		fixture := newSpotTradeRouterUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindPageByOwner(gomock.Any(), signedInViewerID, vo.TradeListFilterVo{
			Status: "closed", Symbol: "2330", Market: "taiwanStock", Limit: 20,
		}).Return([]entities.SpotTradeRecord{}, int64(0), nil)
		fixture.tradingStrategyRepository.EXPECT().FindAllByOwner(gomock.Any(), signedInViewerID).Return(nil, nil)

		response := fixture.send(http.MethodGet, "/spot-trade-records?status=closed&symbol=2330&market=taiwanStock&limit=many", ``)

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"totalCount":0`)
	})

	t.Run("an unknown market answers 400 and statistics answer by market", func(t *testing.T) {
		fixture := newSpotTradeRouterUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindClosedByOwner(gomock.Any(), signedInViewerID, gomock.Nil()).Return(nil, nil)

		unknownMarket := fixture.send(http.MethodGet, "/spot-trade-records?market=futures", ``)
		statistics := fixture.send(http.MethodGet, "/spot-trade-records/statistics?period=all", ``)
		unknownPeriod := fixture.send(http.MethodGet, "/spot-trade-records/statistics?period=1y", ``)

		assert.Equal(t, http.StatusBadRequest, unknownMarket.Code)
		assert.Equal(t, http.StatusBadRequest, unknownPeriod.Code)
		assert.Equal(t, http.StatusOK, statistics.Code)
		assert.Contains(t, statistics.Body.String(), `"market":"taiwanStock"`)
		assert.Contains(t, statistics.Body.String(), `"market":"crypto"`)
	})

	t.Run("a spot journal link answers what it fills in, and a forgotten one 404", func(t *testing.T) {
		fixture := newSpotTradeRouterUnderTest(t)
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "abc").
			Return(entities.StrategyBotRunRecord{StrategyBotID: 3, Result: "sell",
				ReferencePrice: decimal.NewNullDecimal(decimal.RequireFromString("1100"))}, true, nil)
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "gone").
			Return(entities.StrategyBotRunRecord{}, false, nil)
		fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
			Return(entities.StrategyBot{ID: 3, OwnerID: signedInViewerID, Symbol: "2330", MarketDataKind: "kCandle"}, nil)
		fixture.spotTradeRecordRepository.EXPECT().FindOpenByOwnerSymbol(gomock.Any(), signedInViewerID, "2330").
			Return(entities.SpotTradeRecord{}, false, nil)

		prefill := fixture.send(http.MethodGet, "/spot-trade-records/journal-links/abc", ``)
		forgotten := fixture.send(http.MethodGet, "/spot-trade-records/journal-links/gone", ``)

		assert.Equal(t, http.StatusOK, prefill.Code)
		assert.Contains(t, prefill.Body.String(), `"mode":"noOpenHolding"`)
		assert.Equal(t, http.StatusNotFound, forgotten.Code)
	})

	t.Run("a trade answers 200, somebody else's 404, a failed read 502 and a bad identifier 400", func(t *testing.T) {
		fixture := newSpotTradeRouterUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(aHeldSpotTradeOf(signedInViewerID), nil)
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(6)).Return(aHeldSpotTradeOf(signedInViewerID+1), nil)
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(entities.SpotTradeRecord{}, context.DeadlineExceeded)

		own := fixture.send(http.MethodGet, "/spot-trade-records/5", ``)
		strangers := fixture.send(http.MethodGet, "/spot-trade-records/6", ``)
		failed := fixture.send(http.MethodGet, "/spot-trade-records/7", ``)
		badIdentifier := fixture.send(http.MethodGet, "/spot-trade-records/0", ``)

		assert.Equal(t, http.StatusOK, own.Code)
		assert.Equal(t, http.StatusNotFound, strangers.Code)
		assert.Equal(t, http.StatusBadGateway, failed.Code)
		assert.Equal(t, http.StatusBadRequest, badIdentifier.Code)
	})

	t.Run("a comparison answers 200 with nothing closed, 404 for somebody else's strategy and 400 for a bad identifier", func(t *testing.T) {
		fixture := newSpotTradeRouterUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindClosedByOwnerAndTradingStrategy(gomock.Any(), signedInViewerID, gomock.Any()).
			Return(nil, nil).Times(2)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(41)).
			Return(entities.TradingStrategy{ID: 41, OwnerID: signedInViewerID, Name: "台股均線"}, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(42)).
			Return(entities.TradingStrategy{ID: 42, OwnerID: signedInViewerID + 1}, nil)

		nothingClosed := fixture.send(http.MethodGet, "/trading-strategies/41/spot-trade-comparison", ``)
		strangers := fixture.send(http.MethodGet, "/trading-strategies/42/spot-trade-comparison", ``)
		badIdentifier := fixture.send(http.MethodGet, "/trading-strategies/x/spot-trade-comparison", ``)

		assert.Equal(t, http.StatusOK, nothingClosed.Code)
		assert.Contains(t, nothingClosed.Body.String(), `"noClosedTrades":true`)
		assert.Equal(t, http.StatusNotFound, strangers.Code)
		assert.Equal(t, http.StatusBadRequest, badIdentifier.Code)
	})
}

func TestSpotTradeRouterChanges(t *testing.T) {
	t.Run("each change answers the trade as it now stands", func(t *testing.T) {
		fixture := newSpotTradeRouterUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(aHeldSpotTradeOf(signedInViewerID), nil).AnyTimes()
		fixture.spotTradeRecordRepository.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(echoingSpotSave).AnyTimes()

		requests := []struct {
			method string
			path   string
			body   string
		}{
			{http.MethodPost, "/spot-trade-records/5/fills", `{"kind":"sell","price":"1100","quantity":"400"}`},
			{http.MethodPut, "/spot-trade-records/5/fills/1", `{"kind":"buy","filledAt":"2026-09-27T09:00:00Z","price":"1040","quantity":"1000"}`},
			{http.MethodDelete, "/spot-trade-records/5/fills/2", ``},
			{http.MethodPut, "/spot-trade-records/5/plan", `{"plannedStopLossPrice":"1000","entryReason":"站上季線"}`},
			{http.MethodPost, "/spot-trade-records/5/notes", `{"content":"盤後補記"}`},
			{http.MethodPut, "/spot-trade-records/5/setup-tags", `{"setupTagIds":[]}`},
		}

		for _, request := range requests {
			response := fixture.send(request.method, request.path, request.body)

			assert.Equal(t, http.StatusOK, response.Code, request.path)
			assert.Contains(t, response.Body.String(), `"id":5`, request.path)
		}
	})

	t.Run("reviewing a held trade answers 400, and a bad body or identifier answers 400", func(t *testing.T) {
		fixture := newSpotTradeRouterUnderTest(t)
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(aHeldSpotTradeOf(signedInViewerID), nil).AnyTimes()

		cases := []struct {
			method string
			path   string
			body   string
		}{
			{http.MethodPut, "/spot-trade-records/5/review", `{"executionScore":4}`},
			{http.MethodPut, "/spot-trade-records/5/review", `{`},
			{http.MethodPost, "/spot-trade-records/5/fills", `{`},
			{http.MethodPut, "/spot-trade-records/5/fills/1", `{`},
			{http.MethodPut, "/spot-trade-records/5/plan", `{`},
			{http.MethodPost, "/spot-trade-records/5/notes", `{`},
			{http.MethodPut, "/spot-trade-records/5/setup-tags", `{`},
			{http.MethodPut, "/spot-trade-records/5/fills/0", `{}`},
			{http.MethodDelete, "/spot-trade-records/5/fills/x", ``},
			{http.MethodPost, "/spot-trade-records/0/fills", `{}`},
			{http.MethodPut, "/spot-trade-records/x/fills/1", `{}`},
			{http.MethodDelete, "/spot-trade-records/x/fills/1", ``},
			{http.MethodPut, "/spot-trade-records/x/plan", `{}`},
			{http.MethodPost, "/spot-trade-records/x/notes", `{}`},
			{http.MethodPut, "/spot-trade-records/x/review", `{}`},
			{http.MethodPut, "/spot-trade-records/x/setup-tags", `{}`},
			{http.MethodDelete, "/spot-trade-records/x", ``},
		}

		for _, testCase := range cases {
			response := fixture.send(testCase.method, testCase.path, testCase.body)

			assert.Equal(t, http.StatusBadRequest, response.Code, testCase.method+" "+testCase.path+" "+testCase.body)
		}
	})

	t.Run("a closed trade's plan answers 400 and deleting answers 204 or 502", func(t *testing.T) {
		fixture := newSpotTradeRouterUnderTest(t)
		closed := aHeldSpotTradeOf(signedInViewerID)
		closedAt := tradeRouterNow
		closed.Status = "closed"
		closed.ClosedAt = &closedAt
		fixture.spotTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(closed, nil).Times(3)
		fixture.spotTradeRecordRepository.EXPECT().MarkDeleted(gomock.Any(), uint(5), gomock.Any()).Return(nil)
		fixture.spotTradeRecordRepository.EXPECT().MarkDeleted(gomock.Any(), uint(5), gomock.Any()).Return(context.DeadlineExceeded)

		locked := fixture.send(http.MethodPut, "/spot-trade-records/5/plan", `{"plannedStopLossPrice":"1000"}`)
		deleted := fixture.send(http.MethodDelete, "/spot-trade-records/5", ``)
		failed := fixture.send(http.MethodDelete, "/spot-trade-records/5", ``)

		assert.Equal(t, http.StatusBadRequest, locked.Code)
		assert.Contains(t, locked.Body.String(), "平倉後計畫已鎖定")
		assert.Equal(t, http.StatusNoContent, deleted.Code)
		assert.Equal(t, http.StatusBadGateway, failed.Code)
	})
}
