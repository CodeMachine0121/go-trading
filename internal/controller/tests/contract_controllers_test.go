package controller_test

import (
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
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

const contractLookbackCeilingDays = 3650

// assertAnError stands in for anything below the controller breaking, which every
// handler has to report as a bad gateway rather than as the caller's fault.
var assertAnError = errors.New("storage unreachable")

// validContractBody is a complete contract candle: every figure, the trade count and
// all four mark prices, index prices and premium index figures.
const validContractBody = `{"symbol":"BTCUSDT","openTime":"2026-08-29T09:00:00Z",
"open":"100","high":"120","low":"90","close":"120",
"volume":"11","quoteVolume":"1200","takerBuyBaseVolume":"5","takerBuyQuoteVolume":"600",
"tradeCount":7,"markOpen":"101","markHigh":"121","markLow":"91","markClose":"111",
"indexOpen":"102","indexHigh":"122","indexLow":"92","indexClose":"112",
"premiumIndexOpen":"-0.0001","premiumIndexHigh":"0.0002","premiumIndexLow":"-0.0003","premiumIndexClose":"0.0001"}`

func contractCandleAt(openTime time.Time, closePrice string) entities.KCandleContract {
	return entities.KCandleContract{
		Symbol: "BTCUSDT", OpenTime: openTime,
		Open:       decimal.RequireFromString("100"),
		High:       decimal.RequireFromString("120"),
		Low:        decimal.RequireFromString("90"),
		Close:      decimal.RequireFromString(closePrice),
		MarkClose:  decimal.RequireFromString("111"),
		TradeCount: 7,
	}
}

type contractRouterUnderTest struct {
	engine            *gin.Engine
	candleRepository  *mocks.MockIKCandleContractRepository
	symbolRepository  *mocks.MockIContractTradingSymbolRepository
	syncRunRepository *mocks.MockIKCandleContractHistorySyncRunRepository
	marketDataProxy   *mocks.MockIContractMarketDataProxy
	lookupProxy       *mocks.MockIContractSymbolLookupProxy
}

func newContractRouterUnderTest(t *testing.T) contractRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	candleRepository := mocks.NewMockIKCandleContractRepository(mockController)
	symbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	syncRunRepository := mocks.NewMockIKCandleContractHistorySyncRunRepository(mockController)
	marketDataProxy := mocks.NewMockIContractMarketDataProxy(mockController)
	lookupProxy := mocks.NewMockIContractSymbolLookupProxy(mockController)
	settlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(mockController)
	fundingRateProxy := mocks.NewMockIContractFundingRateProxy(mockController)
	statisticRepository := mocks.NewMockIContractPositionStatisticRepository(mockController)
	statisticProxy := mocks.NewMockIContractPositionStatisticProxy(mockController)
	tierProxy := mocks.NewMockIContractMaintenanceMarginTierProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(at(12, 0)).AnyTimes()
	clockProxy.EXPECT().Sleep(gomock.Any()).AnyTimes()

	// A history sync fills in the position statistics after the candles; these tests
	// are about the candles and the answers, so the archive has no day at all.
	archiveProxy := mocks.NewMockIContractPositionStatisticArchiveProxy(mockController)
	archiveProxy.EXPECT().FetchDailyPositionStatistics(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, false, nil).AnyTimes()
	statisticRepository.EXPECT().CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	positionStatisticService := service.NewContractPositionStatisticService(
		statisticRepository, symbolRepository, statisticProxy, archiveProxy, clockProxy, queryMaxResults)
	ingestionService := service.NewContractKCandleIngestionService(
		candleRepository, syncRunRepository, symbolRepository, marketDataProxy, clockProxy,
		domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
		5, 24*time.Hour, positionStatisticService)
	ingestionApplication := application.NewKCandleContractIngestionApplication(ingestionService)

	candleController := controller.NewKCandleContractController(
		application.NewKCandleContractApplication(
			service.NewKCandleContractService(candleRepository, clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}), queryMaxResults)))
	symbolController := controller.NewContractTradingSymbolController(
		application.NewContractTradingSymbolApplication(
			service.NewContractTradingSymbolService(symbolRepository, candleRepository, lookupProxy, clockProxy),
			ingestionService,
			service.NewContractFundingRateService(
				settlementRepository, symbolRepository, fundingRateProxy, clockProxy, queryMaxResults),
			positionStatisticService,
			service.NewContractMaintenanceMarginTierService(
				mocks.NewMockIContractMaintenanceMarginTierRepository(mockController), symbolRepository,
				tierProxy, clockProxy)))
	tierProxy.EXPECT().FetchMaintenanceMarginLadders(gomock.Any()).
		Return(nil, domains.ErrContractAccountCredentialsMissing).AnyTimes()
	// Joining the watchlist also catches funding rates and position statistics up;
	// these tests are about the candles and the answers, so those two have nothing.
	settlementRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any()).
		Return(entities.ContractFundingRateSettlement{}, false, nil).AnyTimes()
	fundingRateProxy.EXPECT().FetchFundingRateSettlements(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	settlementRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
	statisticRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any()).
		Return(entities.ContractPositionStatistic{}, false, nil).AnyTimes()
	statisticProxy.EXPECT().FetchPositionStatistics(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	statisticRepository.EXPECT().SaveAllIfAbsent(gomock.Any(), gomock.Any()).Return(0, nil).AnyTimes()
	backfillController := controller.NewKCandleContractBackfillController(ingestionApplication)
	historySyncController := controller.NewKCandleContractHistorySyncController(
		ingestionApplication, contractLookbackCeilingDays)

	engine := gin.New()
	engine.POST("/contract-k-candles", candleController.CreateKCandleContract)
	engine.GET("/contract-k-candles", candleController.GetKCandleContractsInRange)
	engine.GET("/contract-k-candles/series", candleController.GetKCandleContractSeries)
	engine.POST("/contract-k-candles/backfill", backfillController.CatchUpSymbol)
	engine.POST("/contract-k-candles/history", historySyncController.StartSymbolHistorySync)
	engine.GET("/contract-k-candles/history/:id", historySyncController.GetSymbolHistorySync)
	engine.GET("/contract-k-candles/:symbol/:openTime", candleController.GetKCandleContract)
	engine.PUT("/contract-k-candles/:symbol/:openTime", candleController.UpdateKCandleContract)
	engine.DELETE("/contract-k-candles/:symbol/:openTime", candleController.DeleteKCandleContract)
	engine.GET("/contract-trading-symbols", symbolController.ListContractTradingSymbols)
	engine.POST("/contract-watchlist", symbolController.AddToWatchlist)
	engine.DELETE("/contract-watchlist/:symbol", symbolController.RemoveFromWatchlist)

	return contractRouterUnderTest{
		engine:            engine,
		candleRepository:  candleRepository,
		symbolRepository:  symbolRepository,
		syncRunRepository: syncRunRepository,
		marketDataProxy:   marketDataProxy,
		lookupProxy:       lookupProxy,
	}
}

func (fixture contractRouterUnderTest) call(
	method string, target string, body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

func TestContractCandleWriteResponses(t *testing.T) {
	t.Run("stores a complete candle and echoes it back", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.candleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			Return(contractCandleAt(at(9, 0), "120"), nil)

		recorder := fixture.call(http.MethodPost, "/contract-k-candles", validContractBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"markClose":"111"`)
		assert.Contains(t, recorder.Body.String(), `"tradeCount":7`)
	})

	t.Run("refuses a candle whose mark price was left out", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		withoutMarkPrice := strings.Replace(validContractBody, `,"markClose":"111"`, "", 1)

		recorder := fixture.call(http.MethodPost, "/contract-k-candles", withoutMarkPrice)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "標記價格不得留白")
	})

	t.Run("refuses a candle whose index price was left out", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		withoutIndexPrice := strings.Replace(validContractBody, `"indexOpen":"102",`, "", 1)

		recorder := fixture.call(http.MethodPost, "/contract-k-candles", withoutIndexPrice)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "指數價格必填")
	})

	t.Run("refuses a candle whose trade count was left out", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		withoutTradeCount := strings.Replace(validContractBody, `"tradeCount":7,`, "", 1)

		recorder := fixture.call(http.MethodPost, "/contract-k-candles", withoutTradeCount)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "成交筆數不得留白")
	})

	t.Run("refuses a body it cannot read", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)

		recorder := fixture.call(http.MethodPost, "/contract-k-candles", "not json")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("reports storage failing as a bad gateway", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.candleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			Return(entities.KCandleContract{}, assertAnError)

		recorder := fixture.call(http.MethodPost, "/contract-k-candles", validContractBody)

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}

func TestContractCandleReadResponses(t *testing.T) {
	t.Run("reads a range earliest first", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.candleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandleContract{contractCandleAt(at(9, 0), "120")}, nil)

		recorder := fixture.call(http.MethodGet,
			"/contract-k-candles?symbol=BTCUSDT&startTime=2026-08-29T09:00:00Z&endTime=2026-08-29T09:05:00Z", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"markClose":"111"`)
		// That candle was stored before the index price and premium index existed.
		assert.Contains(t, recorder.Body.String(), `"indexClose":null`)
		assert.Contains(t, recorder.Body.String(), `"premiumIndexClose":null`)
	})

	t.Run("hands a complete candle out with its index price and premium index", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		completeCandle := contractCandleAt(at(9, 0), "120")
		completeCandle.IndexOpen = decimal.NewNullDecimal(decimal.RequireFromString("102"))
		completeCandle.IndexHigh = decimal.NewNullDecimal(decimal.RequireFromString("122"))
		completeCandle.IndexLow = decimal.NewNullDecimal(decimal.RequireFromString("92"))
		completeCandle.IndexClose = decimal.NewNullDecimal(decimal.RequireFromString("112"))
		completeCandle.PremiumIndexOpen = decimal.NewNullDecimal(decimal.RequireFromString("-0.0001"))
		completeCandle.PremiumIndexHigh = decimal.NewNullDecimal(decimal.RequireFromString("0.0002"))
		completeCandle.PremiumIndexLow = decimal.NewNullDecimal(decimal.RequireFromString("-0.0003"))
		completeCandle.PremiumIndexClose = decimal.NewNullDecimal(decimal.RequireFromString("0.0001"))
		fixture.candleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandleContract{completeCandle}, nil)

		recorder := fixture.call(http.MethodGet,
			"/contract-k-candles?symbol=BTCUSDT&startTime=2026-08-29T09:00:00Z&endTime=2026-08-29T09:05:00Z", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		for _, figure := range []string{
			`"indexOpen":"102"`, `"indexHigh":"122"`, `"indexLow":"92"`, `"indexClose":"112"`,
			`"premiumIndexOpen":"-0.0001"`, `"premiumIndexHigh":"0.0002"`,
			`"premiumIndexLow":"-0.0003"`, `"premiumIndexClose":"0.0001"`,
		} {
			assert.Contains(t, recorder.Body.String(), figure)
		}
	})

	t.Run("refuses a time it cannot read", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)

		startRecorder := fixture.call(http.MethodGet,
			"/contract-k-candles?symbol=BTCUSDT&startTime=yesterday&endTime=2026-08-29T09:05:00Z", "")
		endRecorder := fixture.call(http.MethodGet,
			"/contract-k-candles?symbol=BTCUSDT&startTime=2026-08-29T09:00:00Z&endTime=tomorrow", "")
		openTimeRecorder := fixture.call(http.MethodGet, "/contract-k-candles/BTCUSDT/now", "")

		assert.Equal(t, http.StatusBadRequest, startRecorder.Code)
		assert.Equal(t, http.StatusBadRequest, endRecorder.Code)
		assert.Equal(t, http.StatusBadRequest, openTimeRecorder.Code)
	})

	t.Run("reports a candle nobody stored as not found", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.candleRepository.EXPECT().FindOne(gomock.Any(), "BTCUSDT", at(9, 0)).
			Return(entities.KCandleContract{}, domains.ErrKCandleContractNotFound)

		recorder := fixture.call(http.MethodGet,
			"/contract-k-candles/BTCUSDT/2026-08-29T09:00:00Z", "")

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("reads one stored candle", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.candleRepository.EXPECT().FindOne(gomock.Any(), "BTCUSDT", at(9, 0)).
			Return(contractCandleAt(at(9, 0), "120"), nil)

		recorder := fixture.call(http.MethodGet,
			"/contract-k-candles/BTCUSDT/2026-08-29T09:00:00Z", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"close":"120"`)
	})
}

func TestContractCandleUpdateAndDeleteResponses(t *testing.T) {
	t.Run("changes the candle the path names", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.candleRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
			Return(contractCandleAt(at(9, 0), "120"), nil)

		recorder := fixture.call(http.MethodPut,
			"/contract-k-candles/BTCUSDT/2026-08-29T09:00:00Z", validContractBody)

		assert.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("refuses an update whose premium index was left out, without touching the candle", func(t *testing.T) {
		// No update is expected of storage: the candle keeps the figures it had.
		fixture := newContractRouterUnderTest(t)
		withoutPremiumIndex := strings.Replace(validContractBody, `,"premiumIndexClose":"0.0001"`, "", 1)

		recorder := fixture.call(http.MethodPut,
			"/contract-k-candles/BTCUSDT/2026-08-29T09:00:00Z", withoutPremiumIndex)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "溢價指數必填")
	})

	t.Run("refuses a body naming another candle", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		movedSymbol := strings.Replace(validContractBody, "BTCUSDT", "ETHUSDT", 1)
		movedOpenTime := strings.Replace(validContractBody, "T09:00:00Z", "T09:01:00Z", 1)

		symbolRecorder := fixture.call(http.MethodPut,
			"/contract-k-candles/BTCUSDT/2026-08-29T09:00:00Z", movedSymbol)
		openTimeRecorder := fixture.call(http.MethodPut,
			"/contract-k-candles/BTCUSDT/2026-08-29T09:00:00Z", movedOpenTime)

		assert.Equal(t, http.StatusBadRequest, symbolRecorder.Code)
		assert.Contains(t, symbolRecorder.Body.String(), "不得更換交易標的與起始時間")
		assert.Equal(t, http.StatusBadRequest, openTimeRecorder.Code)
	})

	t.Run("refuses an update body it cannot read", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)

		badBodyRecorder := fixture.call(http.MethodPut,
			"/contract-k-candles/BTCUSDT/2026-08-29T09:00:00Z", "not json")
		badTimeRecorder := fixture.call(http.MethodPut,
			"/contract-k-candles/BTCUSDT/now", validContractBody)

		assert.Equal(t, http.StatusBadRequest, badBodyRecorder.Code)
		assert.Equal(t, http.StatusBadRequest, badTimeRecorder.Code)
	})

	t.Run("deletes the named candle and reports a missing one", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.candleRepository.EXPECT().Delete(gomock.Any(), "BTCUSDT", at(9, 0)).Return(nil)
		fixture.candleRepository.EXPECT().Delete(gomock.Any(), "ETHUSDT", at(9, 0)).
			Return(domains.ErrKCandleContractNotFound)

		deletedRecorder := fixture.call(http.MethodDelete,
			"/contract-k-candles/BTCUSDT/2026-08-29T09:00:00Z", "")
		missingRecorder := fixture.call(http.MethodDelete,
			"/contract-k-candles/ETHUSDT/2026-08-29T09:00:00Z", "")
		badTimeRecorder := fixture.call(http.MethodDelete, "/contract-k-candles/BTCUSDT/now", "")

		assert.Equal(t, http.StatusNoContent, deletedRecorder.Code)
		assert.Equal(t, http.StatusNotFound, missingRecorder.Code)
		assert.Equal(t, http.StatusBadRequest, badTimeRecorder.Code)
	})
}

func TestContractWatchlistResponses(t *testing.T) {
	t.Run("lists what the system knows about", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.symbolRepository.EXPECT().FindAll(gomock.Any()).Return(
			[]entities.ContractTradingSymbol{{Symbol: "BTCUSDT", IsWatched: true}}, nil)
		fixture.candleRepository.EXPECT().FindDistinctSymbols(gomock.Any()).Return([]string{}, nil)

		recorder := fixture.call(http.MethodGet, "/contract-trading-symbols", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"symbol":"BTCUSDT"`)
		assert.Contains(t, recorder.Body.String(), `"isWatched":true`)
	})

	t.Run("reports storage failing as a bad gateway", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.symbolRepository.EXPECT().FindAll(gomock.Any()).Return(nil, assertAnError)

		recorder := fixture.call(http.MethodGet, "/contract-trading-symbols", "")

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})

	t.Run("adds a listed contract and catches it up", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "BTCUSDT").
			Return(vo.ContractSymbolListingVo{IsListed: true}, nil)
		fixture.symbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
		fixture.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
			entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil).Times(3)
		fixture.candleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
			Return([]entities.KCandleContract{}, nil)
		fixture.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
			Return([]vo.ContractMarketKCandleVo{}, nil)

		recorder := fixture.call(http.MethodPost, "/contract-watchlist", `{"symbol":"BTCUSDT"}`)

		assert.Equal(t, http.StatusNoContent, recorder.Code)
	})

	t.Run("tells the three ways adding can fail apart", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "NOSUCHPAIR").
			Return(vo.ContractSymbolListingVo{}, nil)
		fixture.lookupProxy.EXPECT().LookUpSymbol(gomock.Any(), "ETHUSDT").
			Return(vo.ContractSymbolListingVo{}, assertAnError)

		unlistedRecorder := fixture.call(http.MethodPost, "/contract-watchlist", `{"symbol":"NOSUCHPAIR"}`)
		unreachableRecorder := fixture.call(http.MethodPost, "/contract-watchlist", `{"symbol":"ETHUSDT"}`)
		blankRecorder := fixture.call(http.MethodPost, "/contract-watchlist", `{"symbol":"  "}`)
		unreadableRecorder := fixture.call(http.MethodPost, "/contract-watchlist", "not json")

		assert.Equal(t, http.StatusBadRequest, unlistedRecorder.Code)
		assert.Equal(t, http.StatusServiceUnavailable, unreachableRecorder.Code)
		assert.Equal(t, http.StatusBadRequest, blankRecorder.Code)
		assert.Equal(t, http.StatusBadRequest, unreadableRecorder.Code)
	})

	t.Run("stops following and answers the same when it already had", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
			entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil)
		fixture.symbolRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
		fixture.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "ETHUSDT").Return(
			entities.ContractTradingSymbol{}, false, nil)

		watchedRecorder := fixture.call(http.MethodDelete, "/contract-watchlist/BTCUSDT", "")
		unwatchedRecorder := fixture.call(http.MethodDelete, "/contract-watchlist/ETHUSDT", "")

		assert.Equal(t, http.StatusNoContent, watchedRecorder.Code)
		assert.Equal(t, http.StatusNoContent, unwatchedRecorder.Code)
	})

	t.Run("reports removal failing as a bad gateway", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
			entities.ContractTradingSymbol{}, false, assertAnError)

		recorder := fixture.call(http.MethodDelete, "/contract-watchlist/BTCUSDT", "")

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}

func TestContractBackfillResponses(t *testing.T) {
	t.Run("catches one contract up", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
			entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil)
		fixture.candleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
			Return([]entities.KCandleContract{}, nil)
		fixture.marketDataProxy.EXPECT().FetchKCandles(gomock.Any(), gomock.Any()).
			Return([]vo.ContractMarketKCandleVo{}, nil)

		recorder := fixture.call(http.MethodPost, "/contract-k-candles/backfill", `{"symbol":"BTCUSDT"}`)

		assert.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("tells an unregistered contract apart from a source that would not answer", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
			entities.ContractTradingSymbol{}, false, nil)
		fixture.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "ETHUSDT").Return(
			entities.ContractTradingSymbol{}, false, assertAnError)

		unregisteredRecorder := fixture.call(http.MethodPost,
			"/contract-k-candles/backfill", `{"symbol":"BTCUSDT"}`)
		brokenRecorder := fixture.call(http.MethodPost,
			"/contract-k-candles/backfill", `{"symbol":"ETHUSDT"}`)
		blankRecorder := fixture.call(http.MethodPost,
			"/contract-k-candles/backfill", `{"symbol":"  "}`)
		unreadableRecorder := fixture.call(http.MethodPost, "/contract-k-candles/backfill", "not json")

		assert.Equal(t, http.StatusNotFound, unregisteredRecorder.Code)
		assert.Equal(t, http.StatusBadGateway, brokenRecorder.Code)
		assert.Equal(t, http.StatusBadRequest, blankRecorder.Code)
		assert.Equal(t, http.StatusBadRequest, unreadableRecorder.Code)
	})
}

func TestContractHistorySyncResponses(t *testing.T) {
	t.Run("accepts the request and answers with the run", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
			entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil)
		fixture.syncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ any, syncRun entities.KCandleContractHistorySyncRun,
			) (entities.KCandleContractHistorySyncRun, error) {
				syncRun.ID = 5

				return syncRun, nil
			}).AnyTimes()
		fixture.candleRepository.EXPECT().
			CountInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(100000, nil).AnyTimes()

		recorder := fixture.call(http.MethodPost,
			"/contract-k-candles/history", `{"symbol":"BTCUSDT","lookbackDays":1}`)

		assert.Equal(t, http.StatusAccepted, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"status":"running"`)
	})

	t.Run("tells every way it can be refused apart", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "ETHUSDT").Return(
			entities.ContractTradingSymbol{}, false, nil)
		fixture.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "SOLUSDT").Return(
			entities.ContractTradingSymbol{Symbol: "SOLUSDT", IsWatched: true}, true, nil)
		fixture.syncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(
			entities.KCandleContractHistorySyncRun{},
			domains.KCandleHistorySyncInProgress("SOLUSDT"))

		lookbackRecorder := fixture.call(http.MethodPost,
			"/contract-k-candles/history", `{"symbol":"BTCUSDT","lookbackDays":0}`)
		blankRecorder := fixture.call(http.MethodPost,
			"/contract-k-candles/history", `{"symbol":"  ","lookbackDays":1}`)
		unregisteredRecorder := fixture.call(http.MethodPost,
			"/contract-k-candles/history", `{"symbol":"ETHUSDT","lookbackDays":1}`)
		inProgressRecorder := fixture.call(http.MethodPost,
			"/contract-k-candles/history", `{"symbol":"SOLUSDT","lookbackDays":1}`)
		unreadableRecorder := fixture.call(http.MethodPost, "/contract-k-candles/history", "not json")

		assert.Equal(t, http.StatusBadRequest, lookbackRecorder.Code)
		assert.Equal(t, http.StatusBadRequest, blankRecorder.Code)
		assert.Equal(t, http.StatusNotFound, unregisteredRecorder.Code)
		assert.Equal(t, http.StatusConflict, inProgressRecorder.Code)
		assert.Equal(t, http.StatusBadRequest, unreadableRecorder.Code)
	})

	t.Run("reports storage failing while recording the run as a bad gateway", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.symbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
			entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil)
		fixture.syncRunRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(
			entities.KCandleContractHistorySyncRun{}, assertAnError)

		recorder := fixture.call(http.MethodPost,
			"/contract-k-candles/history", `{"symbol":"BTCUSDT","lookbackDays":1}`)

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})

	t.Run("reads a run back by its number", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.syncRunRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(
			entities.KCandleContractHistorySyncRun{
				ID: 5, Symbol: "BTCUSDT", Status: string(vo.KCandleHistorySyncSucceeded),
				StartedAt: at(9, 0),
			}, true, nil)
		fixture.syncRunRepository.EXPECT().FindOne(gomock.Any(), uint(6)).Return(
			entities.KCandleContractHistorySyncRun{}, false, nil)
		fixture.syncRunRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(
			entities.KCandleContractHistorySyncRun{}, false, assertAnError)

		foundRecorder := fixture.call(http.MethodGet, "/contract-k-candles/history/5", "")
		missingRecorder := fixture.call(http.MethodGet, "/contract-k-candles/history/6", "")
		brokenRecorder := fixture.call(http.MethodGet, "/contract-k-candles/history/7", "")
		unreadableRecorder := fixture.call(http.MethodGet, "/contract-k-candles/history/five", "")

		assert.Equal(t, http.StatusOK, foundRecorder.Code)
		assert.Contains(t, foundRecorder.Body.String(), `"status":"succeeded"`)
		assert.Equal(t, http.StatusNotFound, missingRecorder.Code)
		assert.Equal(t, http.StatusBadGateway, brokenRecorder.Code)
		assert.Equal(t, http.StatusBadRequest, unreadableRecorder.Code)
	})
}

func TestContractCandleFailureResponses(t *testing.T) {
	t.Run("reports a range read failing as a bad gateway", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.candleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, assertAnError)

		recorder := fixture.call(http.MethodGet,
			"/contract-k-candles?symbol=BTCUSDT&startTime=2026-08-29T09:00:00Z&endTime=2026-08-29T09:05:00Z", "")

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})

	t.Run("reports a range it cannot read as a bad request", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)

		recorder := fixture.call(http.MethodGet,
			"/contract-k-candles?symbol=&startTime=2026-08-29T09:00:00Z&endTime=2026-08-29T09:05:00Z", "")

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("reports an update failing as a bad gateway", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.candleRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
			Return(entities.KCandleContract{}, assertAnError)

		recorder := fixture.call(http.MethodPut,
			"/contract-k-candles/BTCUSDT/2026-08-29T09:00:00Z", validContractBody)

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}

func TestContractCandleSeriesResponses(t *testing.T) {
	const aStretch = "startTime=2026-08-29T09:00:00Z&endTime=2026-08-29T09:09:00Z"

	t.Run("merges a stretch and says which interval it used", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		first, second := contractCandleAt(at(9, 0), "110"), contractCandleAt(at(9, 1), "111")
		fixture.candleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandleContract{first, second}, nil)

		recorder := fixture.call(http.MethodGet, "/contract-k-candles/series?symbol=BTCUSDT&interval=5m&"+aStretch, "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"interval":"5m"`)
		assert.Contains(t, recorder.Body.String(), `"close":"111"`)
		assert.Contains(t, recorder.Body.String(), `"tradeCount":14`)
		assert.Contains(t, recorder.Body.String(), `"markClose":"111"`)
	})

	t.Run("picks an interval from how many the caller can show", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.candleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandleContract{}, nil)

		recorder := fixture.call(http.MethodGet,
			"/contract-k-candles/series?symbol=BTCUSDT&displayableCandleCount=100"+
				"&startTime=2026-08-29T00:00:00Z&endTime=2026-08-29T23:59:00Z", "")

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"interval":"15m"`)
		assert.Contains(t, recorder.Body.String(), `"kCandles":[]`)
	})

	t.Run("refuses what it cannot read or answer", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)

		unreadableCount := fixture.call(http.MethodGet,
			"/contract-k-candles/series?symbol=BTCUSDT&displayableCandleCount=many&"+aStretch, "")
		bothWays := fixture.call(http.MethodGet,
			"/contract-k-candles/series?symbol=BTCUSDT&interval=5m&displayableCandleCount=10&"+aStretch, "")
		noSymbol := fixture.call(http.MethodGet, "/contract-k-candles/series?"+aStretch, "")
		unreadableStart := fixture.call(http.MethodGet,
			"/contract-k-candles/series?symbol=BTCUSDT&startTime=x&endTime=2026-08-29T09:09:00Z", "")
		unreadableEnd := fixture.call(http.MethodGet,
			"/contract-k-candles/series?symbol=BTCUSDT&startTime=2026-08-29T09:00:00Z&endTime=x", "")

		assert.Equal(t, http.StatusBadRequest, unreadableCount.Code)
		assert.Contains(t, unreadableCount.Body.String(), "displayableCandleCount 必須是整數")
		assert.Equal(t, http.StatusBadRequest, bothWays.Code)
		assert.Contains(t, bothWays.Body.String(), "只能挑一種說法")
		assert.Equal(t, http.StatusBadRequest, noSymbol.Code)
		assert.Contains(t, noSymbol.Body.String(), "必須指定交易標的")
		assert.Equal(t, http.StatusBadRequest, unreadableStart.Code)
		assert.Equal(t, http.StatusBadRequest, unreadableEnd.Code)
	})

	t.Run("reports storage failing as a bad gateway", func(t *testing.T) {
		fixture := newContractRouterUnderTest(t)
		fixture.candleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, assertAnError)

		recorder := fixture.call(http.MethodGet, "/contract-k-candles/series?symbol=BTCUSDT&interval=5m&"+aStretch, "")

		assert.Equal(t, http.StatusBadGateway, recorder.Code)
	})
}
