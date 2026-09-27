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

var tradeRouterNow = time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

type contractTradeRouterUnderTest struct {
	engine                         *gin.Engine
	contractTradeRecordRepository  *mocks.MockIContractTradeRecordRepository
	tradeTagRepository             *mocks.MockITradeTagRepository
	tradingStrategyRepository      *mocks.MockITradingStrategyRepository
	strategyBotRunRecordRepository *mocks.MockIStrategyBotRunRecordRepository
	tradeJournalSettingRepository  *mocks.MockITradeJournalSettingRepository
	strategyBotRepository          *mocks.MockIStrategyBotRepository
}

func newContractTradeRouterUnderTest(t *testing.T) contractTradeRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(tradeRouterNow).AnyTimes()
	fixture := contractTradeRouterUnderTest{
		contractTradeRecordRepository:  mocks.NewMockIContractTradeRecordRepository(mockController),
		tradeTagRepository:             mocks.NewMockITradeTagRepository(mockController),
		tradingStrategyRepository:      mocks.NewMockITradingStrategyRepository(mockController),
		strategyBotRunRecordRepository: mocks.NewMockIStrategyBotRunRecordRepository(mockController),
		tradeJournalSettingRepository:  mocks.NewMockITradeJournalSettingRepository(mockController),
		strategyBotRepository:          mocks.NewMockIStrategyBotRepository(mockController),
	}
	contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.ContractTradingSymbol{}, true, nil).AnyTimes()
	maintenanceMarginTierRepository := mocks.NewMockIContractMaintenanceMarginTierRepository(mockController)
	maintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	fundingRateSettlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(mockController)
	fundingRateSettlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	kCandleContractRepository := mocks.NewMockIKCandleContractRepository(mockController)
	kCandleContractRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	kCandleContractRepository.EXPECT().FindPriceExtremesInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(vo.PriceExtremesVo{}, nil).AnyTimes()
	fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), gomock.Any()).
		Return(entities.TradeJournalSetting{}, false, nil).AnyTimes()

	journalService := service.NewContractTradeJournalService(
		fixture.contractTradeRecordRepository, fixture.tradeTagRepository, fixture.tradeJournalSettingRepository,
		fixture.tradingStrategyRepository, contractTradingSymbolRepository, maintenanceMarginTierRepository,
		fundingRateSettlementRepository, kCandleContractRepository, clockProxy)
	recordController := controller.NewContractTradeRecordController(
		application.NewContractTradeJournalApplication(journalService, service.NewTradeJournalLinkService(
			mocks.NewMockIOpaqueIdentifierProxy(mockController), fixture.strategyBotRunRecordRepository,
			fixture.strategyBotRepository, "https://console.example")),
		application.NewContractTradeLiveComparisonApplication(
			journalService, service.NewTradingStrategyService(fixture.tradingStrategyRepository),
			service.NewStrategyScriptService(
				mocks.NewMockIStrategyScriptRepository(mockController),
				mocks.NewMockIPublishedStrategyScriptRepository(mockController)),
			service.NewContractBacktestService(
				kCandleContractRepository, fundingRateSettlementRepository,
				mocks.NewMockIContractPositionStatisticRepository(mockController), contractTradingSymbolRepository,
				maintenanceMarginTierRepository, mocks.NewMockIContractIndicatorScriptProxy(mockController),
				clockProxy, 1000, time.Minute)),
	)

	requiresSignIn := doorOpenFor(t, signedInViewerID)
	engine := gin.New()
	engine.POST("/contract-trade-records", requiresSignIn, recordController.RecordTrade)
	engine.GET("/contract-trade-records", requiresSignIn, recordController.ListTrades)
	engine.GET("/contract-trade-records/statistics", requiresSignIn, recordController.GetStatistics)
	engine.GET("/contract-trade-records/journal-links/:identifier", requiresSignIn, recordController.PrepareJournalLink)
	engine.GET("/contract-trade-records/:id", requiresSignIn, recordController.GetTrade)
	engine.DELETE("/contract-trade-records/:id", requiresSignIn, recordController.DeleteTrade)
	engine.POST("/contract-trade-records/:id/fills", requiresSignIn, recordController.AddFill)
	engine.PUT("/contract-trade-records/:id/fills/:fillId", requiresSignIn, recordController.AmendFill)
	engine.DELETE("/contract-trade-records/:id/fills/:fillId", requiresSignIn, recordController.RemoveFill)
	engine.PUT("/contract-trade-records/:id/plan", requiresSignIn, recordController.AmendPlan)
	engine.POST("/contract-trade-records/:id/notes", requiresSignIn, recordController.AddNote)
	engine.PUT("/contract-trade-records/:id/review", requiresSignIn, recordController.WriteReview)
	engine.PUT("/contract-trade-records/:id/setup-tags", requiresSignIn, recordController.AssignSetupTags)
	engine.GET("/trading-strategies/:id/contract-trade-comparison", requiresSignIn, recordController.CompareWithBacktest)
	fixture.engine = engine

	return fixture
}

func (fixture contractTradeRouterUnderTest) send(method string, path string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	response := httptest.NewRecorder()
	fixture.engine.ServeHTTP(response, request)

	return response
}

func anOpenTradeOf(ownerID uint) entities.ContractTradeRecord {
	openedAt := tradeRouterNow.Add(-time.Hour)

	return entities.ContractTradeRecord{
		ID: 27, OwnerID: ownerID, Symbol: "BTCUSDT", Direction: "long", Leverage: decimal.NewFromInt(10),
		Status: "open", OpenedAt: openedAt,
		Fills: []entities.ContractTradeFill{{
			ID: 1, Kind: "entry", FilledAt: openedAt, Price: decimal.RequireFromString("97905"),
			Quantity: decimal.RequireFromString("0.03"), Liquidity: "taker",
		}},
	}
}

func echoingSave(_ context.Context, record entities.ContractTradeRecord) (entities.ContractTradeRecord, error) {
	return record, nil
}

func TestContractTradeRouterRecordTrade(t *testing.T) {
	t.Run("a trade is recorded and answered with the json field names", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
		fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.ContractTradeRecord{}, false, nil)
		fixture.contractTradeRecordRepository.EXPECT().Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, record entities.ContractTradeRecord) (entities.ContractTradeRecord, error) {
				record.ID = 31
				return record, nil
			})

		response := fixture.send(http.MethodPost, "/contract-trade-records", `{
			"symbol": "BTCUSDT", "direction": "long", "leverage": "10",
			"firstEntryFill": {"kind": "entry", "filledAt": "2026-09-27T09:00:00Z", "price": "97905", "quantity": "0.03"},
			"plan": {"plannedStopLossPrice": "96380", "confidence": 3}
		}`)

		require.Equal(t, http.StatusCreated, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `"id":31`)
		assert.Contains(t, body, `"status":"open"`)
		assert.Contains(t, body, `"plannedStopLossPrice":"96380"`)
		assert.Contains(t, body, `"rMultipleUnavailableReason":""`)
		assert.Contains(t, body, `"feeRateMissing":true`)
	})

	t.Run("a body that is not json, a rule break and a second open trade answer 400, 400 and 409", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil).Times(2)
		fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(anOpenTradeOf(signedInViewerID), true, nil)

		notJson := fixture.send(http.MethodPost, "/contract-trade-records", `{`)
		ruleBreak := fixture.send(http.MethodPost, "/contract-trade-records",
			`{"symbol":"BTCUSDT","direction":"平多","firstEntryFill":{"kind":"entry","price":"1","quantity":"1"}}`)
		second := fixture.send(http.MethodPost, "/contract-trade-records",
			`{"symbol":"BTCUSDT","direction":"long","firstEntryFill":{"kind":"entry","price":"1","quantity":"1"}}`)

		assert.Equal(t, http.StatusBadRequest, notJson.Code)
		assert.Equal(t, http.StatusBadRequest, ruleBreak.Code)
		assert.Contains(t, ruleBreak.Body.String(), "方向只有做多")
		assert.Equal(t, http.StatusConflict, second.Code)
		assert.Contains(t, second.Body.String(), "#27")
		assert.Contains(t, second.Body.String(), `"openTradeId":27`)
	})
}

func TestContractTradeRouterReportsARacingSecondTradeWithoutAnIdentifier(t *testing.T) {
	fixture := newContractTradeRouterUnderTest(t)
	fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil)
	fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entities.ContractTradeRecord{}, false, nil)
	fixture.contractTradeRecordRepository.EXPECT().Create(gomock.Any(), gomock.Any()).
		Return(entities.ContractTradeRecord{}, domains.ContractTradeOpenPositionExists("BTCUSDT", "做多", 0))

	response := fixture.send(http.MethodPost, "/contract-trade-records",
		`{"symbol":"BTCUSDT","direction":"long","firstEntryFill":{"kind":"entry","price":"1","quantity":"1"}}`)

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.NotContains(t, response.Body.String(), "openTradeId")
}

func TestContractTradeRouterReads(t *testing.T) {
	t.Run("the list narrows by the query and falls back on an unreadable limit", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)
		fixture.contractTradeRecordRepository.EXPECT().FindPageByOwner(gomock.Any(), signedInViewerID, vo.TradeListFilterVo{
			Status: "closed", Symbol: "ETHUSDT", Limit: 20,
		}).Return([]entities.ContractTradeRecord{}, int64(0), nil)
		fixture.tradingStrategyRepository.EXPECT().FindAllByOwner(gomock.Any(), signedInViewerID).Return(nil, nil)

		response := fixture.send(http.MethodGet, "/contract-trade-records?status=closed&symbol=ethusdt&limit=many", "")

		require.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t, `{"trades":[],"totalCount":0}`, response.Body.String())
	})

	t.Run("an unknown status or period answers 400", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)

		status := fixture.send(http.MethodGet, "/contract-trade-records?status=planned", "")
		period := fixture.send(http.MethodGet, "/contract-trade-records/statistics?period=1y", "")

		assert.Equal(t, http.StatusBadRequest, status.Code)
		assert.Equal(t, http.StatusBadRequest, period.Code)
	})

	t.Run("statistics answer for the period asked", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)
		fixture.contractTradeRecordRepository.EXPECT().FindClosedByOwner(gomock.Any(), signedInViewerID, nil).Return(nil, nil)

		response := fixture.send(http.MethodGet, "/contract-trade-records/statistics?period=all", "")

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"period":"all"`)
		assert.Contains(t, response.Body.String(), `"winRate":null`)
	})

	t.Run("a journal link answers what it fills in", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "k3y").
			Return(entities.StrategyBotRunRecord{StrategyBotID: 3, RunNumber: 412, SuggestedDirection: "long"}, true, nil)
		fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
			Return(entities.StrategyBot{ID: 3, OwnerID: signedInViewerID, Symbol: "BTCUSDT",
				MarketDataKind: string(vo.MarketDataKindContractKCandle)}, nil)
		fixture.contractTradeRecordRepository.EXPECT().FindOpenByOwnerSymbolDirection(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.ContractTradeRecord{}, false, nil)

		response := fixture.send(http.MethodGet, "/contract-trade-records/journal-links/k3y", "")

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"mode":"newTrade"`)
		assert.Contains(t, response.Body.String(), `"runNumber":412`)
	})

	t.Run("a forgotten journal link answers 404", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)
		fixture.strategyBotRunRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "gone").
			Return(entities.StrategyBotRunRecord{}, false, nil)

		response := fixture.send(http.MethodGet, "/contract-trade-records/journal-links/gone", "")

		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.Contains(t, response.Body.String(), "這一輪的建議已不在紀錄中")
	})

	t.Run("a trade answers 200, somebody else's 404, a failed read 502 and a bad identifier 400", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(anOpenTradeOf(signedInViewerID), nil)
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(28)).Return(anOpenTradeOf(99), nil)
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(29)).Return(entities.ContractTradeRecord{}, context.DeadlineExceeded)

		found := fixture.send(http.MethodGet, "/contract-trade-records/27", "")
		strangers := fixture.send(http.MethodGet, "/contract-trade-records/28", "")
		failed := fixture.send(http.MethodGet, "/contract-trade-records/29", "")
		unreadable := fixture.send(http.MethodGet, "/contract-trade-records/abc", "")

		assert.Equal(t, http.StatusOK, found.Code)
		assert.Contains(t, found.Body.String(), `"symbol":"BTCUSDT"`)
		assert.Equal(t, http.StatusNotFound, strangers.Code)
		assert.Equal(t, http.StatusBadGateway, failed.Code)
		assert.Equal(t, http.StatusBadRequest, unreadable.Code)
	})

	t.Run("a comparison for a strategy that is not the person's answers 404", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)
		fixture.contractTradeRecordRepository.EXPECT().FindClosedByOwnerAndTradingStrategy(gomock.Any(), signedInViewerID, uint(12)).Return(nil, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(12)).Return(entities.TradingStrategy{}, domains.TradingStrategyNotFound(12))

		response := fixture.send(http.MethodGet, "/trading-strategies/12/contract-trade-comparison", "")
		unreadable := fixture.send(http.MethodGet, "/trading-strategies/zero/contract-trade-comparison", "")

		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.Equal(t, http.StatusBadRequest, unreadable.Code)
	})

	t.Run("a comparison with nothing closed answers 200", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)
		fixture.contractTradeRecordRepository.EXPECT().FindClosedByOwnerAndTradingStrategy(gomock.Any(), signedInViewerID, uint(12)).Return(nil, nil)
		fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(12)).
			Return(entities.TradingStrategy{ID: 12, OwnerID: signedInViewerID, Name: "BTC 趨勢跟隨", MarketDataKind: "contractKCandle"}, nil)

		response := fixture.send(http.MethodGet, "/trading-strategies/12/contract-trade-comparison", "")

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"noClosedTrades":true`)
	})
}

func TestContractTradeRouterChanges(t *testing.T) {
	t.Run("each change answers the trade as it now stands", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)
		stored := anOpenTradeOf(signedInViewerID)
		stored.Fills = append(stored.Fills, entities.ContractTradeFill{
			ID: 2, Kind: "entry", FilledAt: stored.OpenedAt.Add(time.Minute), Price: decimal.RequireFromString("97960"),
			Quantity: decimal.RequireFromString("0.021"), Liquidity: "taker",
		})
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(stored, nil).AnyTimes()
		fixture.contractTradeRecordRepository.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(echoingSave).AnyTimes()
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

		requests := []struct {
			method string
			path   string
			body   string
		}{
			{http.MethodPost, "/contract-trade-records/27/fills", `{"kind":"entry","price":"97000","quantity":"0.01"}`},
			{http.MethodPut, "/contract-trade-records/27/fills/1", `{"kind":"entry","filledAt":"2026-09-27T09:00:00Z","price":"97900","quantity":"0.03"}`},
			{http.MethodDelete, "/contract-trade-records/27/fills/2", ``},
			{http.MethodPut, "/contract-trade-records/27/plan", `{"plannedStopLossPrice":"96380","entryReason":"突破"}`},
			{http.MethodPost, "/contract-trade-records/27/notes", `{"content":"加碼太急"}`},
			{http.MethodPut, "/contract-trade-records/27/setup-tags", `{"setupTagIds":[]}`},
		}

		for _, request := range requests {
			response := fixture.send(request.method, request.path, request.body)

			assert.Equal(t, http.StatusOK, response.Code, request.path)
			assert.Contains(t, response.Body.String(), `"id":27`, request.path)
		}
	})

	t.Run("reviewing an open trade answers 400, and a bad body or identifier answers 400", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(anOpenTradeOf(signedInViewerID), nil).AnyTimes()
		fixture.tradeTagRepository.EXPECT().FindByIDs(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

		cases := []struct {
			method string
			path   string
			body   string
		}{
			{http.MethodPut, "/contract-trade-records/27/review", `{"executionScore":4}`},
			{http.MethodPut, "/contract-trade-records/27/review", `{`},
			{http.MethodPost, "/contract-trade-records/27/fills", `{`},
			{http.MethodPut, "/contract-trade-records/27/fills/1", `{`},
			{http.MethodPut, "/contract-trade-records/27/plan", `{`},
			{http.MethodPost, "/contract-trade-records/27/notes", `{`},
			{http.MethodPut, "/contract-trade-records/27/setup-tags", `{`},
			{http.MethodPut, "/contract-trade-records/27/fills/0", `{}`},
			{http.MethodDelete, "/contract-trade-records/27/fills/x", ``},
			{http.MethodPost, "/contract-trade-records/0/fills", `{}`},
			{http.MethodPut, "/contract-trade-records/x/fills/1", `{}`},
			{http.MethodDelete, "/contract-trade-records/x/fills/1", ``},
			{http.MethodPut, "/contract-trade-records/x/plan", `{}`},
			{http.MethodPost, "/contract-trade-records/x/notes", `{}`},
			{http.MethodPut, "/contract-trade-records/x/review", `{}`},
			{http.MethodPut, "/contract-trade-records/x/setup-tags", `{}`},
			{http.MethodDelete, "/contract-trade-records/x", ``},
		}

		for _, testCase := range cases {
			response := fixture.send(testCase.method, testCase.path, testCase.body)

			assert.Equal(t, http.StatusBadRequest, response.Code, testCase.method+" "+testCase.path+" "+testCase.body)
		}
	})

	t.Run("a closed trade's plan answers 400 and deleting answers 204", func(t *testing.T) {
		fixture := newContractTradeRouterUnderTest(t)
		closed := anOpenTradeOf(signedInViewerID)
		closedAt := tradeRouterNow
		closed.Status = "closed"
		closed.ClosedAt = &closedAt
		fixture.contractTradeRecordRepository.EXPECT().FindOne(gomock.Any(), uint(27)).Return(closed, nil).Times(3)
		fixture.contractTradeRecordRepository.EXPECT().MarkDeleted(gomock.Any(), uint(27), gomock.Any()).Return(nil)
		fixture.contractTradeRecordRepository.EXPECT().MarkDeleted(gomock.Any(), uint(27), gomock.Any()).Return(context.DeadlineExceeded)

		locked := fixture.send(http.MethodPut, "/contract-trade-records/27/plan", `{"plannedStopLossPrice":"96380"}`)
		deleted := fixture.send(http.MethodDelete, "/contract-trade-records/27", ``)
		failed := fixture.send(http.MethodDelete, "/contract-trade-records/27", ``)

		assert.Equal(t, http.StatusBadRequest, locked.Code)
		assert.Contains(t, locked.Body.String(), "平倉後計畫已鎖定")
		assert.Equal(t, http.StatusNoContent, deleted.Code)
		assert.Equal(t, http.StatusBadGateway, failed.Code)
	})
}
